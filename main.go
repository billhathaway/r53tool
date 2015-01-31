package main

// This tool is designed to be used by operations to add or remove IP addresses from AWS Route53 record sets

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

const defaultRegion = "us-east-1"

type cli struct {
	r53     *route53.Route53
	log     *log.Logger
	verbose bool
}

// zoneIDByName takes a record name and returns the Route53 zone ID
// TODO: handle paging
func (c *cli) zoneIDByName(name string) (string, error) {
	resp, err := c.r53.ListHostedZones(&route53.ListHostedZonesRequest{})
	if err != nil {
		return "", err
	}
	components := strings.Split(name, ".")
	if len(components) < 3 {
		return "", fmt.Errorf("name must have at least one period")
	}
	name = strings.Join(components[len(components)-3:], ".")
	for _, zone := range resp.HostedZones {
		if *zone.Name == name {
			// zone.ID looks like /hostedzone/Z22CR2RGPPKRQB but we just want the last part
			components := strings.Split(*zone.ID, "/")
			if len(components) != 3 {
				return "", fmt.Errorf("problem splitting id from %s\n", *zone.ID)
			}
			zoneID := components[len(components)-1]
			if c.verbose {
				c.log.Printf("zoneName=%s zoneID=%s\n", name, zoneID)
			}
			return zoneID, nil
		}
	}
	return "", fmt.Errorf("zone %s not found", name)
}

// printResourceRecordSet is a pretty printer
func printResourceRecordSet(rrs route53.ResourceRecordSet) {
	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("", "  ")
	enc.Encode(rrs)
	log.Println()
}

// addToARecordResourceRecordSet adds one or more IP addresses to the Resource Record Set
func (c *cli) addToARecordResourceRecordSet(zoneID string, rrs route53.ResourceRecordSet, ips ...string) error {
	if len(ips) == 0 {
		return fmt.Errorf("at least one IP needs to be passed")
	}
	req := &route53.ChangeResourceRecordSetsRequest{HostedZoneID: aws.String(zoneID)}
	for _, ip := range ips {
		rrs.ResourceRecords = append(rrs.ResourceRecords, route53.ResourceRecord{Value: aws.String(ip)})
	}
	change := route53.Change{Action: aws.String("UPSERT"), ResourceRecordSet: &rrs}
	changeBatch := route53.ChangeBatch{Changes: []route53.Change{change}}
	req.ChangeBatch = &changeBatch
	resp, err := c.r53.ChangeResourceRecordSets(req)
	if err != nil {
		return err
	}
	if c.verbose {
		c.log.Printf("ChangeResourceRecordSets response=%+v\n", resp)
	}
	return nil
}

// getResourceRecordSet finds an existing resource record set matching the criteria
func (c *cli) getResourceRecordSet(zoneID string, recordName string, recordType string, setID string) (route53.ResourceRecordSet, error) {
	zoneID, err := c.zoneIDByName(recordName)
	if err != nil {
		return route53.ResourceRecordSet{}, err
	}
	req := route53.ListResourceRecordSetsRequest{HostedZoneID: &zoneID}
	req.StartRecordName = aws.String(recordName)
	req.StartRecordType = aws.String(recordType)
	resp, err := c.r53.ListResourceRecordSets(&req)
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
	Usage: r53tool [options] ipaddr <ipaddr2 ipaddr3 ...>

	options
	--
	-name="record.example.com.": record name
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
	recordName := flag.String("name", "", "record name")
	recordType := flag.String("type", "A", "record type")
	setID := flag.String("setid", "", "record set identifier")
	region := flag.String("region", defaultRegion, "AWS region")
	verbose := flag.Bool("v", false, "verbose")
	flag.Parse()

	ips := flag.Args()
	if len(ips) == 0 {
		usage()
	}
	c := &cli{
		log:     log.New(os.Stderr, "", log.LstdFlags),
		verbose: *verbose,
	}
	auth, err := aws.EnvCreds()

	if err != nil {
		c.log.Fatal(err)

	}

	c.r53 = route53.New(auth, *region, http.DefaultClient)

	if !strings.HasSuffix(*recordName, ".") {
		*recordName += "."
	}

	zoneID, err := c.zoneIDByName(*recordName)
	if err != nil {
		log.Fatal(err)
	}

	if c.verbose {
		c.log.Printf("zoneID for %s is %s", *recordName, zoneID)
	}

	rrs, err := c.getResourceRecordSet(zoneID, *recordName, *recordType, *setID)
	if err != nil {
		c.log.Fatal(err)
	}

	if c.verbose {
		printResourceRecordSet(rrs)
	}

	err = c.addToARecordResourceRecordSet(zoneID, rrs, ips...)
	if err != nil {
		c.log.Fatal(err)
	}
}
