# Godfather

Transfers DNS zone file records into Route53. Is designed to replicate DNS zone files from a primary or secondary nameserver and create, update, or delete records in Route53. This is to fill the need of Route53 not supporting secondary slave zones, and also to mimic split brain / split horizon DNS with private zones in Route53.

This alleviates the need for using a managed instances of AD or BIND whose sole purpose may be only for DNS. It also removes a point of failure, attack vector, and does so while keeping the process out of band of production critical traffic. 

Initially started as a CLI tool that could have flags passed for domain name, target zone, and R53 zoneID. It has morphed into a lambda function to run in a serverless fashion with a infrastructure as code pipeline.

This is done through a process of a AXFR request to receive a copy of the DNS Zone file, parsing the values into an array and using the Route53 SDK to create or update the records in Route53 public or private zone files.

Additional functionality will be added to support deletion of records. 

## Quickstart

* `cp example.values.yml extra-values.yml`
* Fill out values
* `npm install -g serverless`
* `dep ensure`
* `GOOS=linux go build -o main main.go`
* `serverless deploy`

## Server Test
The DNS server you are using for the AXFR request will need to allow secondary zone transfers. Many Bind or AD DNS instances require IP address specific ACL's for this. You will need to ensure the AXFR request is successful from your VPC's IP address range.

You can test this using DIG or telnet.

"dig example.com axfr" or use a specific DNS server "dig AXFR example.com @127.0.0.1"


## Requirements
Local golang environment installed to build go packages.
A local NODE install for NPM.
Need to install "Serverless" so you won't need to manually create the lambda function in your AWS account.
May need a local copy of the github 3rd party packages.
Install dep via instructions below. 


## extra-values.yml
These values will be specific to your AWS account. You will need to use the VPC specific ID's from your environment to ensure the serverless builds the lambda function properly.



## Contributing

1. Install dep via instructions found [here](https://github.com/golang/dep)
2. Clone the repository using `go get -ud github.com/CreditCardsCom/godfather`
3. Run `dep ensure` in the new package directory

## License (MIT)

Copyright (c) 2018 CreditCards.com

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
