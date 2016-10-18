Running the docker image -- copy setenv-template to setenv, customize
as per your set up, then

<pre>
docker run -p 8000:8000 --env-file ./setenv  xtracdev/atompub --linkhost localhost:8000 --listenaddr :8000
</pre>
