# Production Defaults Example

This example demonstrates the enterprise-ready defaults in the go-rabbitmq library.

## What This Example Shows

- **Quorum Queues by Default**: All queues are created as quorum queues for high availability
- **Automatic DLQ**: Dead Letter Queues are created automatically with sensible defaults
- **Easy Customization**: Simple options to customize quorum and DLQ settings
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

1. **production-queue** - Default quorum queue with DLQ
   - Queue type: Quorum
   - DLX: `production-queue.dlx`
   - DLQ: `production-queue.dlq` (7-day TTL)

2. **custom-quorum** - Customized quorum queue
   - Quorum size: 5 nodes
   - Delivery limit: 3 attempts
   - DLQ TTL: 24 hours

3. **legacy-classic** - Classic queue for compatibility
   - Queue type: Classic
   - Still has DLQ enabled
   - Message TTL: 1 hour

4. **no-dlq-queue** - Queue without DLQ (edge case)
   - Queue type: Quorum
   - No dead letter handling

5. **explicit-quorum** - Using DeclareQuorumQueue method
   - Equivalent to default behavior now
   - Custom delivery limit: 5

## Key Benefits Demonstrated

- **Zero Configuration**: Production-ready queues with no setup
- **High Availability**: Quorum consensus prevents data loss
- **Poison Message Protection**: Delivery limits prevent infinite loops
- **Error Handling**: Failed messages automatically routed to DLQ
- **Easy Migration**: Legacy behavior available via simple options

## Inspecting Results

You can view the created queues in the RabbitMQ Management UI at <http://localhost:15672> (guest/guest).

Notice how:

- Queues show as "quorum" type with replication
- DLX and DLQ are automatically created and bound
- Arguments show the quorum and DLQ configurations
