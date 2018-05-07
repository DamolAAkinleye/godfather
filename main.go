package main

import (
	"fmt"
	"log"
	"reflect"
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

//Lambda passes the necessary items through a JSON string provided given the three below items.
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

		destRecords, err := getDestinationRecords(svc, zone, event)

		if err != nil {
			log.Fatalf("Error fetching destination records: %s\n", err)
		}

		records, err := acquireTransferRecords(zone, event.Master)

		if err != nil {
			log.Fatalf("Error fetching records: %s\n", err)
		}

		if err := replicateRecords(svc, records, event, destRecords); err != nil {
			log.Printf("Error replicating zone %s: %s\n", zone, err)
		}
	}
	//End AD zone pull
}

// Transfer records from the dns zone `z` and nameserver `ns`
// returning an array of all resource records
func acquireTransferRecords(z string, ns string) ([]dns.RR, error) {
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

	//fmt.Println(records[11].Header().Ttl)
	return records, nil

}

// TODO: This requires quite a bit of cleanup
func replicateRecords(svc *route53.Route53, rs []dns.RR, event LambdaRule, destRecords map[string]*route53.ResourceRecordSet) error {
	var changes []*route53.Change
	fmt.Println(len(destRecords))
	for _, record := range rs {
		rrs := &route53.ResourceRecordSet{
			TTL:  aws.Int64(int64(record.Header().Ttl)), //TTL Handler from RR_Header miekg DNS package
			Name: aws.String(record.Header().Name),
		}
		fmt.Println(reflect.TypeOf((record)))

		switch t := interface{}(record).(type) {
		case *dns.A:
			populateRecordSet(rrs, &destRecords, record.Header().Name, "A", t.A.String())

		case *dns.CNAME:
			populateRecordSet(rrs, &destRecords, record.Header().Name, "CNAME", t.Target)

		case *dns.MX:
			populateRecordSet(rrs, &destRecords, record.Header().Name, "MX", t.Mx)

		case *dns.TXT:

			value := fmt.Sprintf("\"%s\"", strings.Join(t.Txt, " "))
			populateRecordSet(rrs, &destRecords, record.Header().Name, "TXT", value)

		default:
			//key := record.Header().Name + ":" + destRecords //Maybe ??
			//key := record.Header().Name + ":" + record.Header().

			//if _, ok := (destRecords)[key]; ok {
			//	delete(destRecords, key)
			//}

			continue
		}

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
	fmt.Println(len(destRecords))
	// TODO: Turn this into a loop on 500 record chunks
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
func populateRecordSet(rr *route53.ResourceRecordSet, destRecords *map[string]*route53.ResourceRecordSet, recName string, recType string, recVal string) {

	key := recName + ":" + recType
	if _, ok := (*destRecords)[key]; ok {
		delete(*destRecords, key)
	}

	rr.SetType(recType)
	rr.ResourceRecords = []*route53.ResourceRecord{
		{
			Value: aws.String(recVal),
		},
	}

}

func getDestinationRecords(svc *route53.Route53, zoneName string, event LambdaRule) (map[string]*route53.ResourceRecordSet, error) {
	DestinationRecords := make(map[string]*route53.ResourceRecordSet)
	err := populateDestinationMap(svc, zoneName, event.ZoneID, nil, &DestinationRecords)
	return DestinationRecords, err
}
func populateDestinationMap(svc *route53.Route53, zoneName string, ZoneID string, respList2 *route53.ListResourceRecordSetsOutput, DestinationRecords *map[string]*route53.ResourceRecordSet) error {

	var listParams = &route53.ListResourceRecordSetsInput{}

	listParams = &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(ZoneID), // Required
		MaxItems:     aws.String("100")}

	if respList2 != nil {
		listParams = &route53.ListResourceRecordSetsInput{
			HostedZoneId:          aws.String(ZoneID), // Required
			MaxItems:              aws.String("100"),
			StartRecordIdentifier: respList2.NextRecordIdentifier,
			StartRecordName:       respList2.NextRecordName,
			StartRecordType:       respList2.NextRecordType,
		}
	}
	respList, err := svc.ListResourceRecordSets(listParams)
	for _, record := range respList.ResourceRecordSets {
		key1 := *record.Name + ":" + *record.Type
		(*DestinationRecords)[key1] = record
	}
	if *respList.IsTruncated {
		err = populateDestinationMap(svc, zoneName, ZoneID, respList, DestinationRecords)
	}

	return err
}

func isDuplicateRecord(a *route53.ResourceRecordSet, b *route53.ResourceRecordSet) bool {
	return *a.Name == *b.Name && *a.Type == *b.Type
}
