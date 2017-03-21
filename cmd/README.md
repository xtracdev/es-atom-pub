Running the docker image -- copy setenv-template to setenv, customize
as per your set up, then

<pre>
docker run -p 8000:8000 --env-file ./setenv  xtracdev/atompub --linkhost localhost:8000 --listenaddr :8000
</pre>

For secure configuration, set up a CMK is AWS KMS, and set your KEY_ALIAS
environment variable to the key alias on AWS. You will need to set 
the AWS_REGION and AWS_PROFILE environment variables for the KMS. For 
insecure configuration, omit KEY_ALIAS or set it to the empty string.

Never use insecure configuration for production usage, and use it just
for developer convenience and unit testing.