terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.31"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.13"
    }
  }
  backend "s3" {
    bucket         = "spectrum-terraform-state"
    key            = "terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "spectrum-terraform-locks"
  }
}

provider "aws" {
  region = var.aws_region
}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", var.cluster_name]
  }
}

provider "helm" {
  kubernetes {
    host                   = module.eks.cluster_endpoint
    cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)
    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "aws"
      args        = ["eks", "get-token", "--cluster-name", var.cluster_name]
    }
  }
}

# ── VPC Module ─────────────────────────────────────────────────────
module "vpc" {
  source = "./modules/vpc"

  name       = "spectrum-vpc"
  cidr_block = "10.0.0.0/16"

  azs             = var.availability_zones
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  enable_nat_gateway = true
  single_nat_gateway = false

  tags = var.tags
}

# ── EKS Cluster ────────────────────────────────────────────────────
module "eks" {
  source = "./modules/eks"

  cluster_name    = var.cluster_name
  cluster_version = var.kubernetes_version
  vpc_id          = module.vpc.vpc_id
  subnet_ids      = module.vpc.private_subnets

  node_groups = {
    core = {
      instance_types = ["t3.xlarge"]
      desired_size   = 3
      min_size       = 3
      max_size       = 6
      disk_size      = 100
      labels = {
        role = "core"
      }
    }
    observability = {
      instance_types = ["t3.2xlarge"]
      desired_size   = 2
      min_size       = 2
      max_size       = 5
      disk_size      = 200
      labels = {
        role = "observability"
      }
    }
  }

  tags = var.tags
}

# ── EKS Add-ons ────────────────────────────────────────────────────
resource "aws_eks_addon" "ebs_csi" {
  cluster_name = module.eks.cluster_name
  addon_name   = "aws-ebs-csi-driver"
}

resource "aws_eks_addon" "coredns" {
  cluster_name = module.eks.cluster_name
  addon_name   = "coredns"
}

# ── Elasticache Redis ─────────────────────────────────────────────────
resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "spectrum-redis"
  engine               = "redis"
  node_type            = "cache.t3.micro"
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  port                 = 6379
  subnet_group_name    = aws_elasticache_subnet_group.redis.name
  security_group_ids   = [aws_security_group.redis.id]

  tags = var.tags
}

resource "aws_elasticache_subnet_group" "redis" {
  name       = "spectrum-redis-subnet"
  subnet_ids = module.vpc.private_subnets
}

resource "aws_security_group" "redis" {
  name   = "spectrum-redis-sg"
  vpc_id = module.vpc.vpc_id

  ingress {
    from_port   = 6379
    to_port     = 6379
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }
}

# ── MSK Kafka ──────────────────────────────────────────────────────
resource "aws_msk_cluster" "kafka" {
  cluster_name           = "spectrum-kafka"
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 3

  broker_node_group_info {
    instance_type   = "kafka.t3.small"
    ebs_volume_size = 100
    client_subnets  = module.vpc.private_subnets
    security_groups = [aws_security_group.kafka.id]
  }

  tags = var.tags
}

resource "aws_security_group" "kafka" {
  name   = "spectrum-kafka-sg"
  vpc_id = module.vpc.vpc_id

  ingress {
    from_port   = 9092
    to_port     = 9094
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/16"]
  }
}

# ── Helm Deploy ────────────────────────────────────────────────────
resource "helm_release" "spectrum" {
  name       = "spectrum"
  namespace  = "observability"
  chart      = "../helm/spectrum"
  depends_on = [module.eks]

  set {
    name  = "global.environment"
    value = var.environment
  }

  set {
    name  = "aiEngine.llmBackend"
    value = var.llm_backend
  }

  set {
    name  = "apiGateway.jwtSecret"
    value = var.jwt_secret
  }
}

# ── Grafana Ingress ────────────────────────────────────────────────
resource "kubernetes_ingress_v1" "grafana" {
  metadata {
    name      = "grafana"
    namespace = "observability"
    annotations = {
      "cert-manager.io/cluster-issuer" = "letsencrypt-prod"
      "nginx.ingress.kubernetes.io/ssl-redirect" = "true"
    }
  }
  spec {
    ingress_class_name = "nginx"
    tls {
      hosts       = ["grafana.${var.domain_name}"]
      secret_name = "grafana-tls"
    }
    rule {
      host = "grafana.${var.domain_name}"
      http {
        path {
          path = "/"
          backend {
            service {
              name = "grafana"
              port {
                number = 3000
              }
            }
          }
        }
      }
    }
  }
}
