Running the docker image -- copy setenv-template to setenv, customize
as per your set up, then

<pre>
docker run -p 8000:8000 --env-file ./setenv  xtracdev/atompub --linkhost localhost:8000 --listenaddr :8000
</pre>

For secure configuration, INSECURE_PUBLISHER must be set to 0 in the 
environment if provided, and the paths to the private key, certificate, and CA
certificate must be provided using the PRIVATE_KEY, CERTIFICATE, and 
CACERT environment variables, respectively.

To run without MTLS configured (for local testing purposes only, definitely
no for production), export INSECURE_PUBLISHER with the value 1.

For test purposes, [this README](https://github.com/d-smith/go-examples/tree/master/mtls-proxy) 
has information on how to generate
a CACERT and create certificates and keys for testing.

For the full secure TLS and proxy config:

* Generate a CA cert
* Generate a server cert with the atomfeedpub CN
* Generate a proxy server cert with the nginxproxy CN
* Generate a client cert with the replicator CN

Note if you do the above, you'll need to alias nginxproxy to localhost
in your /etc/hosts file or you'll get a cert mismatch error if 
you call the service using localhost. Alternatively you can generate
a server cert for localhost and reference that in rp.conf, or
generate a cert with your hostname and reference that, etc.