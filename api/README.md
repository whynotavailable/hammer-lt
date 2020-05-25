The api project contains the controller for the system
It's purpose is to take requests for running and managing tests
and turn them into requests against etcd which acts as the control plane.

The ultimate goal will be that the UI drives the API, and the API responds to the UI in one of two ways.

* Actual REST requests (for initial page loads)
* Websockets (for updates in real time)

The goal is that everything in the UI updates based on the page you are on, so you'll get contextual updates
to the UI instead of all of them. This means that the UI will need to tell the API (through the socket)
which page it's on. This will make it so that you can scale out the UI, API, and agents individually. The only real
thing that'll be annoying in that model is cleaning up old servers, but that's idempotent, so it really doesn't matter.