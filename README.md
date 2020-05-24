## types of messages

/lt/server/{server id} is server health monitoring the format should be

```json
{
  "id": "{uuid}"
}
```

/lt/test/{lease id} is a test that needs to be run, the lease will last for 1 hour, and it's included so writes go there.

```json
{
  "state": "start|stop|pause",
  "servers": ["{serverid1}", "{serverid2}"],
  "targets": [
    {
      "uri": "",
      "method": "GET",
      "headers": {
        "x-my-header": ["value"]
      },
      "body": ""
    }
  ]
}
```

/lt/results/{lease id}/{server id} these are the results from the server, to be sent to the clients, they are under the lease so they expire.

```json
{
  "requests": 4000,
  "statusCodes": {
    "200": 12
  },
  "p50": 3,
  "p90": 60,
  "p99": 120
}
```

the api will watch /lt/ and the nodes will watch /lt/test/

Eventually I'll set it up so that the expiration is runtime+30 minutes for the lease.

## Websockets

From a websockets perspective, what's in the gorilla sockets example for chat is great, I'd go a bit further and track what page the user is on.
This will allow filtered messaging to target specific pages (think results not going to everyone).

I'm going to start with just an API, and add websockets as functionality comes on line. The websockets
will return the same data as the API, but in real time, you will need to let the server know which page you're on
to get relevant information.
