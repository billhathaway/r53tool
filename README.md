# r53tool
CLI to add and remove resource records from route53  BETA BETA BETA might delete everything

This is intended to be an easy way for ops people to add and remove records from existing Route53 resource record sets.  

It is still very early in development which could cause bad things to happen and make you sad and/or fired.  

It also depends on the very unstable auto-generated AWS SDK.

	Usage: r53tool [options] ipaddr <ipaddr2 ipaddr3 ...>  

	options  
	--  
	-name="record.example.com.": record name  
	-region="us-east-1": AWS region  
	-setid="": record set identifier  
	-type="A": record type  
	-v=false: verbose  
  
	This tool will update Route53 record sets with additional resources.  

	Standard AWS environment variables are used to supply authentication credentials.  


