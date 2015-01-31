# r53tool
CLI to add and remove IP resource records from route53 A records

This is intended to be an easy way for ops people to add and remove records from existing Route53 resource record sets. No support yet for health checks.

It is still very early in development which could cause bad things to happen and make you sad and/or fired, but seems to work correctly from my testing.

It also depends on the very unstable auto-generated AWS SDK.

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



