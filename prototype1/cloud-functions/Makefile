API=https://us-central1-apollo-server-273118.cloudfunctions.net/prototype1

local:
	npm start

deploy:
	rm -f prototype1.zip
	cd src && zip -r ../prototype1.zip *
	gsutil cp prototype1.zip gs://apollo-server-273118-deploy/prototype1.zip

clean:
	rm -f prototype1.zip

infection1:
	curl -d '{"method":"infection","ids":["12345678"]}' -H "Content-Type: application/json" \
           -X POST ${API}

scan1:
	curl -d '{"method":"scan","prefixes":["1234"]}' -H "Content-Type: application/json" \
           -X POST ${API}

corsquery1:
	curl -v -X OPTIONS ${API}
