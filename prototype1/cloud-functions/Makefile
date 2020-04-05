local:
	npm start

deploy:
	rm -f prototype1.zip
	cd src && zip -r ../prototype1.zip *
	gsutil cp prototype1.zip gs://apollo-server-273118-deploy/prototype1.zip

clean:
	rm -f prototype1.zip
