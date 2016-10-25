Running the docker image -- copy setenv-template to setenv, customize
as per your set up, then

<pre>
docker run -p 8000:8000 --env-file ./setenv  xtracdev/atompub --linkhost localhost:8000 --listenaddr :8000
</pre>

For secure configuration, INSECURE_PUBLISHER must be set to 1 in the 
environment, and the paths to the private key, certificate, and CA
certificate must be provided using the PRIVATE_KEY, CERTIFICATE, and 
CACERT environment variables, respectively.

For test purposes, [this README](https://github.com/d-smith/go-examples/tree/master/mtls-proxy) 
has information on how to generate
a CACERT and create certificates and keys for testing.