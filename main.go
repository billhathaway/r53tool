package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/gen/route53"
)

var region = "us-east-1"

// awsString turns the pointers back to strings without needless nil checking each time
func awsString(val aws.StringValue) string {
	if val == nil {
		return ""
	}
	return *val
}

// zoneIDByName takes a record name and returns the Route53 zone ID
// TODO: handle paging
func zoneIDByName(r53 *route53.Route53, name string) (string, error) {
	resp, err := r53.ListHostedZones(&route53.ListHostedZonesRequest{})
	if err != nil {
		return "", err
	}
	c := strings.Split(name, ".")
	if len(c) < 3 {
		return "", fmt.Errorf("name must have at least one period")
	}
	name = strings.Join(c[len(c)-3:], ".")
	for _, zone := range resp.HostedZones {
		if *zone.Name == name {
			// zone.ID looks like /hostedzone/Z22CR2RGPPKRQB but we just want the last part
			c := strings.Split(*zone.ID, "/")
			if len(c) != 3 {
				return "", fmt.Errorf("problem splitting id from %s\n", *zone.ID)
			}
			return c[len(c)-1], nil
		}
	}
	return "", fmt.Errorf("zone %s not found", name)
}

// printResourceRecordSet is a pretty printer
func printResourceRecordSet(rrs route53.ResourceRecordSet) {
	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("", "  ")
	enc.Encode(rrs)
	fmt.Println()
}

// addToARecordResourceRecordSet adds one or more IP addresses to the Resource Record Set
func addToARecordResourceRecordSet(r53 *route53.Route53, zoneID string, rrs route53.ResourceRecordSet, ips ...string) error {
	if len(ips) == 0 {
		return fmt.Errorf("at least one IP needs to be passed")
	}
	req := &route53.ChangeResourceRecordSetsRequest{HostedZoneID: aws.String(zoneID)}
	// change := route53.Change{}
	// change.ResourceRecordSet = &rrs
	printResourceRecordSet(rrs)
	for _, ip := range ips {
		rrs.ResourceRecords = append(rrs.ResourceRecords, route53.ResourceRecord{Value: aws.String(ip)})
	}
	change := route53.Change{Action: aws.String("UPSERT"), ResourceRecordSet: &rrs}
	changeBatch := route53.ChangeBatch{Changes: []route53.Change{change}}
	req.ChangeBatch = &changeBatch
	resp, err := r53.ChangeResourceRecordSets(req)
	if err != nil {
		return err
	}
	log.Printf("ChangeResourceRecordSets response=%+v\n", resp)
	return nil
}

// getResourceRecordSet finds an existing resource record set matching the criteria
func getResourceRecordSet(r53 *route53.Route53, zoneID string, recordName string, recordType string, setID string) (route53.ResourceRecordSet, error) {
	zoneID, err := zoneIDByName(r53, recordName)
	if err != nil {
		return route53.ResourceRecordSet{}, err
	}
	req := route53.ListResourceRecordSetsRequest{HostedZoneID: &zoneID}
	req.StartRecordName = aws.String(recordName)
	req.StartRecordType = aws.String(recordType)
	resp, err := r53.ListResourceRecordSets(&req)
	if err != nil {
		return route53.ResourceRecordSet{}, err
	}

	for _, rrs := range resp.ResourceRecordSets {
		if *rrs.Name == recordName && *rrs.SetIdentifier == setID {
			return rrs, nil
		}
	}
	return route53.ResourceRecordSet{}, fmt.Errorf("no ResourceRecordSets found for zoneID=%s recordName=%s recordType=%s setIdentifier=%s\n", zoneID, recordName, recordType, setID)
}

func usage() {
	example := `
	Usage: r53cli [options] ipaddr <ipaddr2 ipaddr3 ...>

	options
	--
	-name="sports.xre.bookmanager.org": record name
	-region="us-east-1": AWS region
	-setid="": record set identifier
	-type="A": record type
	-v=false: verbose

	This tool will update Route53 record sets with additional resources.

	Standard AWS environment variables are used to supply authentication credentials

`
	fmt.Println(example)
	os.Exit(1)
}

func main() {
	recordName := flag.String("name", "record.example.com", "record name")
	recordType := flag.String("type", "A", "record type")
	setID := flag.String("setid", "", "record set identifier")
	region := flag.String("region", "us-east-1", "AWS region")
	verbose := flag.Bool("v", false, "verbose")
	flag.Parse()

	ips := flag.Args()
	if len(ips) == 0 {
		usage()
	}

	auth, err := aws.EnvCreds()
	if err != nil {
		log.Fatal(err)
	}

	r53 := route53.New(auth, *region, http.DefaultClient)

	if !strings.HasSuffix(*recordName, ".") {
		*recordName += "."
	}

	zoneID, err := zoneIDByName(r53, *recordName)
	if err != nil {
		log.Fatal(err)
	}

	if *verbose {
		log.Printf("zoneID for %s is %s", *recordName, zoneID)
	}

	rrs, err := getResourceRecordSet(r53, zoneID, *recordName, *recordType, *setID)
	if err != nil {
		log.Fatal(err)
	}

	if *verbose {
		printResourceRecordSet(rrs)
	}

	err = addToARecordResourceRecordSet(r53, zoneID, rrs, ips...)
	if err != nil {
		log.Fatal(err)
	}
}
