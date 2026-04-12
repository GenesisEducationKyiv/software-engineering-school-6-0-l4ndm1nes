output "ec2_public_ip" {
  description = "Public IP of the EC2 instance"
  value       = aws_eip.app.public_ip
}

output "ec2_public_dns" {
  description = "Public DNS of the EC2 instance"
  value       = aws_instance.app.public_dns
}

output "rds_endpoint" {
  description = "RDS PostgreSQL endpoint"
  value       = aws_db_instance.postgres.endpoint
}

output "redis_endpoint" {
  description = "ElastiCache Redis endpoint"
  value       = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:6379"
}

output "ssh_command" {
  description = "SSH command to connect to the instance"
  value       = "ssh -i ${path.module}/${var.ec2_key_name}.pem ec2-user@${aws_eip.app.public_ip}"
}

output "app_url" {
  description = "Application URL"
  value       = "http://${aws_eip.app.public_ip}"
}

output "metrics_url" {
  description = "Prometheus metrics URL"
  value       = "http://${aws_eip.app.public_ip}/metrics"
}

output "grpc_endpoint" {
  description = "gRPC endpoint"
  value       = "${aws_eip.app.public_ip}:50051"
}
