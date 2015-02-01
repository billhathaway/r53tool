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
const version = ".01"

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

func mapKeys(data map[string]struct{}) []string {
	var keys []string
	for k := range data {
		keys = append(keys, k)
	}
	return keys

}

// delFromARecordResourceRecordSet deletes one or more IP addresses from the Resource Record Set
func (c *cli) delFromARecordResourceRecordSet(zoneID string, rrs route53.ResourceRecordSet, ips ...string) error {
	if len(ips) == 0 {
		return fmt.Errorf("at least one IP needs to be passed")
	}

	// put the slice into a map so we can easily determine if an existing record is in our list to delete
	ipMap := make(map[string]struct{})
	for _, ip := range ips {
		ipMap[ip] = struct{}{}
	}
	var newRecords []route53.ResourceRecord

	for _, rr := range rrs.ResourceRecords {
		if _, exists := ipMap[*rr.Value]; exists {
			if c.verbose {
				c.log.Printf("deleting IP %s\n", *rr.Value)
			}
			// don't keep the record and remove it from map so we only keep the keys for entries we didn't delete
			delete(ipMap, *rr.Value)
		} else {
			// keep the record if we didn't have it in our to delete list
			newRecords = append(newRecords, rr)
		}
	}
	rrs.ResourceRecords = newRecords

	if c.verbose && len(ipMap) > 0 {
		c.log.Printf("IPs not found to delete %v\n", mapKeys(ipMap))
	}

	req := &route53.ChangeResourceRecordSetsRequest{HostedZoneID: aws.String(zoneID)}
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

func usageFatal(message string) {
	example := `
	Usage: r53tool [flags] ipaddr <ipaddr2 ipaddr3 ...>

					required flags
					--
					-name="record.example.com.": record name
					-setid="": record set identifier

					optional flags
					--
					-cmd="add" or "del" (defaults to add)
					-v=false: verbose
					-region="us-east-1": AWS region
					-type="A": record type (currently only A is supported)


	This tool will update Route53 resource record sets by adding or removing IPs.
	Currently the resource record sets needs to already exist.

	Standard AWS environment variables are used to supply authentication credentials

	Examples:
	  # adding IPs
		r53tool -name=www.example.com -setid dc1 192.168.1.1 192.168.1.2

		# deleting IPs
		r53tool -cmd=del -name=www.example.com -setid dc1 192.168.1.1 192.168.1.2
`
	fmt.Println(message)
	fmt.Println(example)
	fmt.Println("version", version)
	os.Exit(1)
}

func main() {
	recordName := flag.String("name", "", "record name")
	recordType := flag.String("type", "A", "record type")
	setID := flag.String("setid", "", "record set identifier")
	region := flag.String("region", defaultRegion, "AWS region")
	verbose := flag.Bool("v", false, "verbose")
	action := flag.String("cmd", "add", "add | del - action to take with IPs")
	flag.Parse()
	c := &cli{
		log: log.New(os.Stderr, "", log.LstdFlags),
	}

	ips := flag.Args()
	if len(ips) == 0 {
		usageFatal("ERROR: need one or more ipaddrs")
	}
	c.verbose = *verbose
	switch *action {
	case "add", "del":
	default:
		usageFatal("ERROR: supported cmd values are add | del")
	}

	switch *recordType {
	case "A":
	default:
		usageFatal("ERROR: only operations on A records are currently supported")
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

	rrs, err := c.getResourceRecordSet(zoneID, *recordName, *recordType, *setID)
	if err != nil {
		c.log.Fatal(err)
	}

	if c.verbose {
		printResourceRecordSet(rrs)
	}

	switch *action {
	case "add":
		err = c.addToARecordResourceRecordSet(zoneID, rrs, ips...)
		if err != nil {
			c.log.Fatal(err)
		}
	case "del":
		err = c.delFromARecordResourceRecordSet(zoneID, rrs, ips...)
		if err != nil {
			c.log.Fatal(err)
		}
	default:
		usageFatal("action not implemented " + *action)
	}

}
