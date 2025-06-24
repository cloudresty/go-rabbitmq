# Reconnection Test Example

&nbsp;

This example demonstrates the automatic reconnection capabilities of the go-rabbitmq package. It shows how the package handles connection failures gracefully and automatically reconnects when the RabbitMQ server becomes available again.

&nbsp;

## Features Demonstrated

- **Automatic Reconnection**: Intelligent retry with exponential backoff
- **Connection Monitoring**: Real-time connection status monitoring
- **Message Continuity**: Messages continue to flow after reconnection
- **Error Handling**: Proper error logging during connection issues
- **Statistics Tracking**: Message counts and loss detection

🔝 [back to top](#reconnection-test-example)

&nbsp;

## How to Use

&nbsp;

### 1. Set Up Environment

```bash
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"

# Optional: Configure reconnection behavior
export RABBITMQ_AUTO_RECONNECT=true
export RABBITMQ_RECONNECT_DELAY=5s
export RABBITMQ_MAX_RECONNECT_ATTEMPTS=0  # 0 = unlimited
export RABBITMQ_HEARTBEAT=10s
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

### 2. Run the Example

```bash
go run examples/reconnection-test/main.go
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

### 3. Test Reconnection

The example will start publishing and consuming messages every 2 seconds. To test reconnection:

🔝 [back to top](#reconnection-test-example)

&nbsp;

#### Option A: Stop/Start RabbitMQ Server

```bash
# Stop RabbitMQ
sudo systemctl stop rabbitmq-server
# or
sudo service rabbitmq-server stop
# or (Docker)
docker stop rabbitmq

# Wait and observe the logs - you'll see connection errors

# Start RabbitMQ again
sudo systemctl start rabbitmq-server
# or
sudo service rabbitmq-server start
# or (Docker)
docker start rabbitmq
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

#### Option B: Restart RabbitMQ

```bash
sudo systemctl restart rabbitmq-server
# or
docker restart rabbitmq
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

#### Option C: Network Disruption

```bash
# Block RabbitMQ port temporarily
sudo iptables -A INPUT -p tcp --dport 5672 -j DROP

# Wait and observe reconnection attempts

# Restore connection
sudo iptables -D INPUT -p tcp --dport 5672 -j DROP
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

## What You'll See

&nbsp;

### Normal Operation

```json
{"level":"info","message":"Message published successfully","message_number":1,"published_count":1}
{"level":"info","message":"Message consumed","consumed_count":1,"message_size":44}
{"level":"info","message":"Connection status","publisher_connected":true,"consumer_connected":true}
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

### During Connection Failure

```json
{"level":"error","message":"Failed to publish message","message_number":5,"error":"connection failed"}
{"level":"warn","message":"Connection issues detected","publisher_connected":false,"consumer_connected":false}
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

### During Reconnection

```json
{"level":"info","message":"Attempting to reconnect to RabbitMQ","attempt":1,"delay":"5s"}
{"level":"info","message":"Successfully reconnected to RabbitMQ","attempt":2}
{"level":"info","message":"Message published successfully","message_number":8,"published_count":6}
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

### Final Statistics

```json
{"level":"info","message":"Reconnection test completed","total_published":15,"total_consumed":15,"message_loss":0}
{"level":"info","message":"No message loss detected during reconnection test"}
```

🔝 [back to top](#reconnection-test-example)

&nbsp;

## Configuration Options

You can customize reconnection behavior via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `RABBITMQ_AUTO_RECONNECT` | `true` | Enable automatic reconnection |
| `RABBITMQ_RECONNECT_DELAY` | `5s` | Initial delay between reconnection attempts |
| `RABBITMQ_MAX_RECONNECT_ATTEMPTS` | `0` | Maximum reconnection attempts (0 = unlimited) |
| `RABBITMQ_HEARTBEAT` | `10s` | Connection heartbeat interval |
| `RABBITMQ_DIAL_TIMEOUT` | `30s` | TCP connection timeout |
| `RABBITMQ_CHANNEL_TIMEOUT` | `10s` | AMQP channel timeout |

🔝 [back to top](#reconnection-test-example)

&nbsp;

## Expected Behavior

1. **Immediate Detection**: Connection failures are detected within one heartbeat interval
2. **Automatic Retry**: Reconnection attempts start immediately with exponential backoff
3. **Message Continuity**: Messages resume flowing once connection is restored
4. **Zero Data Loss**: In-flight messages are properly handled during reconnection
5. **Graceful Recovery**: No manual intervention required

🔝 [back to top](#reconnection-test-example)

&nbsp;

## Stopping the Test

Press `Ctrl+C` to stop the test. The example will:

1. Stop publishing new messages
2. Wait for in-flight messages to complete
3. Close connections gracefully
4. Display final statistics including any message loss

This example is perfect for validating your RabbitMQ cluster's high availability setup and understanding how the go-rabbitmq package behaves during network issues or server maintenance.

🔝 [back to top](#reconnection-test-example)

&nbsp;

---

&nbsp;

An open source project brought to you by the [Cloudresty](https://cloudresty.com) team.

[Website](https://cloudresty.com) &nbsp;|&nbsp; [LinkedIn](https://www.linkedin.com/company/cloudresty) &nbsp;|&nbsp; [BlueSky](https://bsky.app/profile/cloudresty.com) &nbsp;|&nbsp; [GitHub](https://github.com/cloudresty)

&nbsp;
