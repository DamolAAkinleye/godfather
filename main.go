package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/miekg/dns"
)

func main() {
	lambda.Start(HandleRequest)
}

type LambdaRule struct {
	ZoneID string `json:"HostedZoneID"`
	Master string `json:"Master"`
	Zone   string `json:"Zone"`
}

// {   "HostedZoneID": "ABC123",   "Master": "10.0.0.1",   "Zone": "derp.com." }

func HandleRequest(event LambdaRule) {

	// Establish AWS session
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("failed to create session,", err)
		return
	}

	svc := route53.New(sess)

	if event.ZoneID == "" || event.Master == "" || event.Zone == "" {
		fmt.Println(fmt.Errorf("Incomplete arguments: %s, %s, %s\n", event.ZoneID, event.Master, event.Zone))
		return
	}

	//Active Directory zones being pulled
	zones := []string{event.Zone}

	for _, zone := range zones {
		records, err := transferRecords(zone, event.Master)

		if err != nil {
			log.Fatalf("Error fetching records: %s\n", err)
		}

		if err := replicateRecords(svc, records, event); err != nil {
			log.Printf("Error replicating zone %s: %s\n", zone, err)
		}
	}
	//End AD zone pull
}

// Transfer records from the dns zone `z` and nameserver `ns`
// returning an array of all resource records
func transferRecords(z string, ns string) ([]dns.RR, error) {
	tx := dns.Transfer{}
	msg := dns.Msg{}
	var records []dns.RR

	msg.SetAxfr(z)
	msg.SetTsig("axrf.", dns.HmacMD5, 300, time.Now().Unix())

	c, err := tx.In(&msg, ns+":53")

	if err != nil {
		return nil, err
	}

	for env := range c {
		if env.Error != nil {
			return records, nil
		}

		records = append(records, env.RR...)
	}

	return records, nil
}

// TODO: This requires quite a bit of cleanup
func replicateRecords(svc *route53.Route53, rs []dns.RR, event LambdaRule) error {
	var changes []*route53.Change
	//So far only supported record types are A CNAME MX and TXT. But could easily add support for record types as needed.
	for _, record := range rs {
		rrs := &route53.ResourceRecordSet{
			TTL:  aws.Int64(60),
			Name: aws.String(record.Header().Name),
		}

		switch t := interface{}(record).(type) {
		case *dns.A:
			rrs.SetType("A")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(t.A.String()),
				},
			}
		case *dns.CNAME:
			rrs.SetType("CNAME")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(t.Target),
				},
			}
		case *dns.MX:
			rrs.SetType("MX")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(t.Mx),
				},
			}
		case *dns.TXT:
			value := fmt.Sprintf("\"%s\"", strings.Join(t.Txt, " "))

			rrs.SetType("TXT")
			rrs.ResourceRecords = []*route53.ResourceRecord{
				{
					Value: aws.String(value),
				},
			}
		default:
			// NOOP
			continue
		}
		//Used UPSERT to create and update instead of create only.
		c := &route53.Change{
			Action:            aws.String("UPSERT"),
			ResourceRecordSet: rrs,
		}

		// Print out any validation errors
		if err := rrs.Validate(); err != nil {
			fmt.Printf("Invalid record: %s, %s", record.Header().Name, err)
		} else {
			if l := len(changes); l > 0 &&
				isDuplicateRecord(changes[l-1].ResourceRecordSet, rrs) {
				previous := changes[l-1].ResourceRecordSet.ResourceRecords

				previous = append(previous, rrs.ResourceRecords...)
			} else {
				changes = append(changes, c)
			}
		}

	}

	// Amazon AWS api limits to 1000 requests per session request. It double counted the list/create so have to chunk it into 500 at a time to stay under the limit.
	wg := sync.WaitGroup{}
	chunkSize := 500
	for chunk := 0; (chunk * chunkSize) < len(changes); chunk++ {
		var bounds int

		if next := chunk*chunkSize + chunkSize; next < len(changes) {
			bounds = next
		} else {
			bounds = len(changes)
		}

		chunkedChanges := changes[chunk*chunkSize : bounds]

		wg.Add(1)
		go makeRoute53Request(svc, chunkedChanges, &wg, event.ZoneID)
	}

	// Wait for requests to finish
	wg.Wait()

	return nil
}

func makeRoute53Request(svc *route53.Route53, changes []*route53.Change, wg *sync.WaitGroup, zoneID string) {
	defer wg.Done()

	cs := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &route53.ChangeBatch{
			Comment: aws.String(time.Now().UTC().String()),
			Changes: changes,
		},
	}
	resp, err := svc.ChangeResourceRecordSets(cs)

	if err != nil {
		log.Printf("Failed to make route53 request: %s\n", err)
		return
	}

	fmt.Printf("Created %d records; %#v\n", len(changes), resp)
}

//If you have multiple A records for a single host entry with IP's this looks for that condition
func isDuplicateRecord(a *route53.ResourceRecordSet, b *route53.ResourceRecordSet) bool {
	return *a.Name == *b.Name && *a.Type == *b.Type
}
