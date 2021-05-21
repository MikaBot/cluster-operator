# Relay Server

The relay server allows your clusters to communicate messages to one another (i.e. custom events)

# Relay dispatch
```json
{
  "type": 0,
  "body": "test entity event"
}
```

# Relay response
```json
{
  "type": 1,
  "body": "test entity event"
}
```

```javascript
// This example assumes you're using port 3010, change it if need be.
// The code to connect to this server and publish a message is below.
// NOTE: You need to properly authenticate or else your connection will be closed.
const WebSocket = require("ws");
// a will be the publisher
const a = new WebSocket("ws://localhost:3010/relay", {
    headers: {
        Authorization: "token"
    }
});

// b will act as a subscriber
const b = new WebSocket("ws://localhost:3010/relay", {
    headers: {
        Authorization: "token"
    }
});

a.on("open", () => {
    console.log("Dispatching message...");
    a.send(JSON.stringify({
        type: 0, // type should be 0 for sending, the relay server will send 1 for received events
        body: "Hello, world!"
    }));
});

b.on("message", m => {
    const { type } = JSON.parse(m);
    if (type === 1) {
        console.log(`Received message: ${m}`);
    }
    // Received message: {"type":1,"body":"Hello, world!"}
});
```