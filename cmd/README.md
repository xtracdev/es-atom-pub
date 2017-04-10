Running the docker image -- copy setenv-template to setenv, customize
as per your set up, then

<pre>
docker run -p 8000:8000 --env-file ./setenv  xtracdev/atompub --linkhost localhost:8000 --listenaddr :8000
</pre>

For secure configuration, set up a CMK is AWS KMS, and set your KEY\_ALIAS
environment variable to the key alias on AWS. You will need to set 
the AWS\_REGION and AWS\_PROFILE environment variables for the KMS (or 
alternatively AWS\_ACCESS\_KEY\_ID and 
AWS\_SECRET\_ACCESS\_KEY. For 
insecure configuration, omit KEY\_ALIAS or set it to the empty string.

Never use insecure configuration for production usage, and use it just
for developer convenience and unit testing.

To test your KMS set up, use recent.go in the util directory. You can inject your AWS credentials in the usual way - either specify an AWS_PROFILE environment variable with
the named profile to picked up credentials, or specify the AWS\_ACCESS\_KEY\_ID and
AWS\_SECRET\_ACCESS\_KEY.