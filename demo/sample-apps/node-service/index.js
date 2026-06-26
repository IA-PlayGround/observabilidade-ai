const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { Resource } = require('@opentelemetry/resources');
const { SemanticResourceAttributes } = require('@opentelemetry/semantic-conventions');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');

const sdk = new NodeSDK({
  resource: new Resource({
    [SemanticResourceAttributes.SERVICE_NAME]: 'catalog',
    [SemanticResourceAttributes.SERVICE_VERSION]: '1.0.0',
    'deployment.environment': 'development',
    'k8s.namespace': 'demo',
  }),
  traceExporter: new OTLPTraceExporter({
    url: 'http://otel-collector:4317',
  }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();

process.on('SIGTERM', () => sdk.shutdown());
process.on('SIGINT', () => sdk.shutdown());

const express = require('express');
const app = express();
const PORT = 8084;

app.get('/products', async (req, res) => {
  await randomDelay(30, 80);
  const products = [
    { id: 1, name: 'Widget', price: 9.99 },
    { id: 2, name: 'Gadget', price: 19.99 },
    { id: 3, name: 'Doodad', price: 29.99 },
  ];

  if (Math.random() < 0.05) {
    res.status(500).json({ error: 'database connection timeout' });
    return;
  }

  res.json(products);
});

app.get('/products/:id', async (req, res) => {
  await randomDelay(10, 40);
  const id = parseInt(req.params.id);
  res.json({ id, name: `Product ${id}`, price: 9.99 + id * 10 });
});

app.get('/health', (req, res) => {
  res.json({ status: 'healthy' });
});

app.listen(PORT, () => {
  console.log(`catalog service listening on :${PORT}`);
});

function randomDelay(min, max) {
  return new Promise(resolve => setTimeout(resolve, min + Math.random() * max));
}
