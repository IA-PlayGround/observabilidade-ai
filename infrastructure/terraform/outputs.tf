output "vpc_id" {
  value = module.vpc.vpc_id
}

output "cluster_endpoint" {
  value     = module.eks.cluster_endpoint
  sensitive = true
}

output "cluster_name" {
  value = module.eks.cluster_name
}

output "redis_endpoint" {
  value = aws_elasticache_cluster.redis.cache_nodes[0].address
}

output "kafka_bootstrap_brokers" {
  value = aws_msk_cluster.kafka.bootstrap_brokers
}

output "grafana_url" {
  value = "https://grafana.${var.domain_name}"
}

output "spectrum_api_gateway_url" {
  value = "https://api.${var.domain_name}"
}
