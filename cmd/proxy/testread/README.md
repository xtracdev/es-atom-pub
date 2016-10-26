This directory contains a simple client file
that may be configured with a client cert signed by the same CA as the
feed server. The client can do a get on provided atomfeed
URI read from the command line.

This is useful for debugging.

The client takes 4 arguments:

1. The client key file
2. The client cert file
3. The CA cert file
4. The uri to GET using TLS configured from the first 3 arguments.
