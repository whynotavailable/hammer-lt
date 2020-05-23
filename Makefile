node:
	cd node &&\
		go build &&\
		node

api:
	cd api &&\
		go build &&\
		api

tester:
	cd tester &&\
		go build &&\
		tester