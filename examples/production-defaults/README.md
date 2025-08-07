# Production Defaults Example

This example demonstrates the production-ready defaults and queue configuration options in the go-rabbitmq library.

## What This Example Shows

- **Quorum Queues by Default**: All queues are created as quorum queues for high availability
- **Manual Dead Letter Configuration**: Full control over dead letter exchange and routing setup
- **Easy Customization**: Simple options to customize quorum settings
- **Legacy Compatibility**: How to opt into classic queue behavior when needed

## Running the Example

1. Ensure RabbitMQ is running locally:

   ```bash
   docker run -d --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3-management
   ```

2. Run the example:

   ```bash
   go run main.go
   ```

## What Gets Created

The example creates several queues to demonstrate different configurations:

1. **production-queue** - Default quorum queue
   - Queue type: Quorum
   - High availability with replication

2. **custom-quorum** - Customized quorum queue
   - Quorum size: 5 nodes
   - Delivery limit: 3 attempts

3. **dlq-example** - Queue with manual dead letter configuration
   - Queue type: Quorum
   - Dead letter exchange: `errors.dlx`
   - Dead letter routing key: `failed`

4. **legacy-classic** - Classic queue for compatibility
   - Queue type: Classic
   - Message TTL: 1 hour

5. **explicit-quorum** - Using DeclareQuorumQueue method
   - Equivalent to default behavior now
   - Custom delivery limit: 5

## Key Benefits

- **High Availability**: Quorum queues provide replication and fault tolerance
- **User Control**: Complete control over topology and dead letter configuration
- **Flexibility**: Mix and match different queue types and configurations as needed

## Inspecting Results

You can view the created queues in the RabbitMQ Management UI at <http://localhost:15672> (guest/guest).

Notice how:

- Queues show as "quorum" type with replication
- DLX and DLQ are automatically created and bound
- Arguments show the quorum and DLQ configurations
