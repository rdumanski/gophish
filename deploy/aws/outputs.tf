output "public_ip" {
  description = "Stable Elastic IP of the instance. Point your phishing domain's A record here later."
  value       = aws_eip.this.public_ip
}

output "ssh_command" {
  description = "SSH into the box."
  value       = "ssh ubuntu@${aws_eip.this.public_ip}"
}

output "admin_tunnel_command" {
  description = "Open an SSH tunnel, then browse https://localhost:3333 for the admin UI."
  value       = "ssh -N -L 3333:localhost:3333 ubuntu@${aws_eip.this.public_ip}"
}

output "get_initial_password_command" {
  description = "Run on the box to retrieve the auto-generated initial admin password."
  value       = "ssh ubuntu@${aws_eip.this.public_ip} 'cd /opt/gophish-src && sudo docker compose -f deploy/aws/docker-compose.yml logs gophish | grep -i password'"
}

output "phish_landing_url" {
  description = "Public phishing/landing server (HTTP). Targets reach this."
  value       = "http://${aws_eip.this.public_ip}/"
}
