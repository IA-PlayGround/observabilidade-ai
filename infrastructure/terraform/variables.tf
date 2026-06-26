variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "production"
}

variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "spectrum-cluster"
}

variable "kubernetes_version" {
  description = "Kubernetes version"
  type        = string
  default     = "1.29"
}

variable "availability_zones" {
  description = "AZs for subnets"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

variable "domain_name" {
  description = "Domain for Grafana ingress"
  type        = string
  default     = "spectrum.dev"
}

variable "llm_backend" {
  description = "Default LLM backend (openai, anthropic, local)"
  type        = string
  default     = "openai"
}

variable "jwt_secret" {
  description = "JWT secret for API Gateway"
  type        = string
  sensitive   = true
  default     = "change-me-in-production"
}

variable "tags" {
  description = "Resource tags"
  type        = map(string)
  default = {
    Project     = "Spectrum"
    Environment = "production"
    ManagedBy   = "Terraform"
  }
}
