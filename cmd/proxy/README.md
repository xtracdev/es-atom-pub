# Proxy Notes - Nginx and Docker

## Context

The es atom publisher serves up an atom feed of event store events. The
event store feeds are immutable as the represent an ordered set of 
events, each of which are immutable. All feeds except the recent
feed are immutable, and the retrieval of a feed or event has
cache headers that are immutable as well. Therefore the use of 
a caching proxy in front of the feed will allow the access of 
feeds to be scaled up.

## Docker

The easiest way to run nginx as a proxy is in a docker container. First some notes...

Docker networks provide the mechanism to allow contains to reference each
other by name.

For this example I created a network named foo:

<pre>
docker network create foo
</pre>

I then ran the atom pub container using this network:

<pre>
docker run -p 4000:8000  --rm --env-file ./setenv   --name atomfeedpub --network foo xtracdev/atompub --linkhost localhost:5000 --listenaddr :8000
</pre>

The atomfeedpub name is significant here: this is the name that nginx will reference
as a host in the reverse proxy configuration.

The container can be seen when inspecting the network:

<pre>
$ docker network inspect foo
[
    {
        "Name": "foo",
        "Id": "e8254fc528db14a22c6bdd727bc4ab7bc17e00d3553287c2416653786c2d9d0d",
        "Scope": "local",
        "Driver": "bridge",
        "EnableIPv6": false,
        "IPAM": {
            "Driver": "default",
            "Options": {},
            "Config": [
                {
                    "Subnet": "172.18.0.0/16",
                    "Gateway": "172.18.0.1/16"
                }
            ]
        },
        "Internal": false,
        "Containers": {
            "0140f78b09ba0258b6ca49f5b5991c7c9b69f2378ed4d67996f3cd0aef57e60c": {
                "Name": "atomfeedpub",
                "EndpointID": "f52bbcdd2ba32a35db805fba7c1db944bb5296934043025bd9647b208f1de5ce",
                "MacAddress": "02:42:ac:12:00:02",
                "IPv4Address": "172.18.0.2/16",
                "IPv6Address": ""
            }
        },
        "Options": {},
        "Labels": {}
    }
]
</pre>

The nginx config includes the atompub container name:

<pre>
events {

}

http {
    proxy_cache_path  /tmp/rpcache  levels=1:2    keys_zone=STATIC:10m
    inactive=24h  max_size=1g;
    server {
        listen       5000;
        location / {
            proxy_pass             http://atomfeedpub:8000;
            proxy_set_header       Host $host;
            proxy_cache            STATIC;
            proxy_cache_valid      200  1d;
            proxy_cache_use_stale  error timeout invalid_header updating
                                   http_500 http_502 http_503 http_504;
        }
    }
}
</pre>

Note the 'native' port in the container is referenced.

With the reverse proxy configuration set, the reverse proxy can be started. Note
that it is run on the same network (foo):

<pre>
docker run --network foo -p 5000:5000 -v $GOPATH/src/github.com/xtraclabs/es-atom-feed-proxy/rp.conf:/etc/nginx/nginx.conf nginx
</pre>

Again, after starting the nginx container, we can see it on the same network

<pre>
$ docker network inspect foo
[
    {
        "Name": "foo",
        "Id": "e8254fc528db14a22c6bdd727bc4ab7bc17e00d3553287c2416653786c2d9d0d",
        "Scope": "local",
        "Driver": "bridge",
        "EnableIPv6": false,
        "IPAM": {
            "Driver": "default",
            "Options": {},
            "Config": [
                {
                    "Subnet": "172.18.0.0/16",
                    "Gateway": "172.18.0.1/16"
                }
            ]
        },
        "Internal": false,
        "Containers": {
            "0140f78b09ba0258b6ca49f5b5991c7c9b69f2378ed4d67996f3cd0aef57e60c": {
                "Name": "atomfeedpub",
                "EndpointID": "f52bbcdd2ba32a35db805fba7c1db944bb5296934043025bd9647b208f1de5ce",
                "MacAddress": "02:42:ac:12:00:02",
                "IPv4Address": "172.18.0.2/16",
                "IPv6Address": ""
            },
            "8ac48e2d25ef5c19017290d37cb54330a03b9d794701a9051875f3d83486ad12": {
                "Name": "elated_mirzakhani",
                "EndpointID": "7f505d2fbcb44f7dda9ec34ea524d5b0bee97ea87d00e4ff7a9103294e0d1aea",
                "MacAddress": "02:42:ac:12:00:03",
                "IPv4Address": "172.18.0.3/16",
                "IPv6Address": ""
            }
        },
        "Options": {},
        "Labels": {}
    }
]
</pre>

Note that when using Docker compose, a docker network for the assembly of containers defined
in the compose file is created automatically on docker-compose up, and is removed
on docker-compose down.

## Docker Compose with Consul

First, run some consul

<pre>
docker run -p 8500:8500 consul agent -dev -client 0.0.0.0
</pre>

Grab envconsul from [this repository](https://github.com/hashicorp/envconsul).

Seed the values needed to demo, e.g.

<pre>
curl -X PUT -d nginxproxy:5000 http://localhost:8500/v1/kv/devcenter/dc1/LINKHOST
curl -X PUT -d :8000 http://localhost:8500/v1/kv/devcenter/dc1/LISTENADDR
curl -X PUT -d esdbo http://localhost:8500/v1/kv/devcenter/dc1/DB_USER
curl -X PUT -d password http://localhost:8500/v1/kv/devcenter/dc1/DB_PASSWORD
curl -X PUT -d 10.24.118.2 http://localhost:8500/v1/kv/devcenter/dc1/DB_HOST
curl -X PUT -d 1521 http://localhost:8500/v1/kv/devcenter/dc1/DB_PORT
curl -X PUT -d xe.oracle.docker http://localhost:8500/v1/kv/devcenter/dc1/DB_SVC
curl -X PUT -d /certs/atomfeedpub.key http://localhost:8500/v1/kv/devcenter/dc1/PRIVATE_KEY
curl -X PUT -d /certs/atomfeedpub.crt http://localhost:8500/v1/kv/devcenter/dc1/CERTIFICATE
curl -X PUT -d /certs/ca.crt http://localhost:8500/v1/kv/devcenter/dc1/CACERT
</pre>

Then run it:

<pre>
./envconsul -prefix devcenter/dc1 env docker-compose up
</pre>
