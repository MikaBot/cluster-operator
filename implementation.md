# Implementation

Here is a guide of how you can learn how to implement all the JSON spec.

# Packet structure

Packets to and from the server have the following structure

| Field | Type | Description |
|-------|------|------|
| type  | number | The packet type, 0 to 11
| body  | any    | The body of the packet

An example packet is provided below.
```json
{
  "type": 1,
  "body": {
    "id": 0,
    "block": {
      "shards": [0],
      "total": 1
    }
  }
}
```

# Handshaking

Handshaking is a broad term here, because for one it's used to define the connection to a WebSocket, and for this operator it's defined to alert the operator this cluster is connecting.

You should send a handshaking packet as soon as the WebSocket opens:
```json
{
  "type": 0
}
```

# Receiving shard data

When you have successfully sent a type 1 payload to the operator, you will now receive an event containing cluster information (such as the ID, and the shards you'll be handling).

It will look like so:
```json
{
  "type": 1,
  "body": {
    "id": 0,
    "block": {
      "shards": [0],
      "total": 1
    }
  }
}
```

When you have this data, and when your client turns ready, send a ready event:
```json
{ "type": 9 }
```

# Pings
Now we move onto the topic of health checking, the default time the operator checks if your cluster is alive is 5 seconds (this cannot be changed).
If you fail to acknowledge a ping, your cluster will be terminated; furthermore, forcing you to reconnect entirely.

You should receive a packet like so:
```json
{ "type": 2 }
```

Then, you need to respond like so:
```json
{ "type": 3 }
```

# Eval
Sometimes you might want to evaluate something on all clusters (generally, this should be restricted to the developers of your bot.)

The field "body.id" should be a randomly generated cryptographic string, to avoid eval ID conflicts.
An example eval payload is below:
```json
{
  "type": 5,
  "body": {
    "id": "1",
    "code": "1 + 1"
  }
}
```

On the cluster side, when you get this payload, you'll need to respond to it.

An optional field `body.error` (string) can be specified to indicate an evaluation error (it is recommended NOT to include `body.res`, but it's up to you.)

### Example
```json
{
  "type": 4,
  "body": {
    "id": "1",
    "res": "2"
  }
}
```

When all clusters have hopefully responded, you will now get a packet with type 6.
NOTE: Should any error occur with eval, instead of the `res` field, your client should send the `error` field instead.

```json
{
  "type": 6,
  "body": {
    "results": [
      {
        "res": "2"
      }
    ]
  }
}
```

The second way to evaluate is using

`POST /eval`

| Header | Value |
|-------|-------|
| Authorization | The token you have in the operators config. |

An example is provided below:
```json
{
  "id": "1",
  "code": "1 + 1",
  "timeout": 5000
}
```

You should then get a response similar to the following:
```json
{
  "data": [
    {
      "res": "2"
    }
  ]
}
```

# Metrics
When prometheus is configured properly, the cluster operator will send a type 7 packet, to collect statistics.

**IMPORTANT NOTE:** There is a special label called `cluster`, when set it will use the cluster ID as the label value.


Assuming, your operator metrics config was
```json
{
    "metrics": [
      {"type": "gauge", "name": "messages", "description": "Messages seen"},
      {"type": "gauge", "name": "commands", "description": "Command usage", "labels":  ["name"]}
    ]
}
```

You should respond like so:
```json
{
  "type": 8,
  "body": {
    "messages": 42,
    "commands": {
      "help": 42
    }
  }
}
```

# Entities
Instead of evaluating data you want, you should use entities, it will be more secure than evaluating the data you want.
**especially if you rely on user input for those entities.**

To get started send a request like so

`POST /entity`

| Header | Value |
|-------|-------|
| Authorization | The token you have in the operators config. |

Again, the `id` field should be a randomly generated ID and an optional property `args` can also be specified, this is an object.

The body should be like so.

```json
{
  "id": "1",
  "type": "hello"
}
```

Now, your client will get a payload similar to (NOTE: If you provided `args` that field will be present.):
```json
{
  "type": 10,
  "body": {
    "id": "1",
    "type": "hello"
  }
}
```

Your client should respond like so.

**NOTE: THE ID MUST MATCH THE ONE IN THE REQUEST, OR THE REQUEST WILL TIMEOUT**
```json
{
  "type": 11,
  "body": {
    "id": "1",
    "data": "world"
  }
}
```

You should now get an HTTP response like so, entries are ordered by cluster ID:
```json
{
  "data": [
    {
      "res": "world"
    },
    {
      "error": "cluster unhealthy"
    }
  ]
}
```