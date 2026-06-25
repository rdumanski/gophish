###############################################################################
# Lookups: default VPC + latest Ubuntu 24.04 LTS AMI from Canonical
###############################################################################

data "aws_vpc" "default" {
  default = true
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

###############################################################################
# Key pair
###############################################################################

resource "aws_key_pair" "this" {
  key_name   = "${var.project_name}-key"
  public_key = var.ssh_public_key
}

###############################################################################
# Security group
#   80/443  -> world  (targets must reach the phishing/landing pages)
#   22      -> allowed_ssh_cidr (also the path to the admin UI via tunnel)
#   3333    -> only if admin_allowed_cidr is set; otherwise reach it over SSH
###############################################################################

resource "aws_security_group" "this" {
  name        = "${var.project_name}-sg"
  description = "gophish: phishing server public, admin locked down"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "SSH (and admin-UI tunnel)"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_ssh_cidr]
  }

  ingress {
    description = "Phishing landing pages (HTTP)"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Open now even though TLS is deferred, so adding Let's Encrypt later
  # needs no security-group change.
  ingress {
    description = "Phishing landing pages (HTTPS)"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  dynamic "ingress" {
    for_each = var.admin_allowed_cidr == "" ? [] : [var.admin_allowed_cidr]
    content {
      description = "Admin UI (direct exposure — opt-in)"
      from_port   = 3333
      to_port     = 3333
      protocol    = "tcp"
      cidr_blocks = [ingress.value]
    }
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.project_name}-sg" }
}

###############################################################################
# EC2 instance
###############################################################################

resource "aws_instance" "this" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.this.key_name
  vpc_security_group_ids = [aws_security_group.this.id]

  user_data = templatefile("${path.module}/user_data.sh.tftpl", {
    repo_url        = var.repo_url
    repo_ref        = var.repo_ref
    contact_address = var.contact_address
  })

  # Re-run cloud-init if the bootstrap script changes (replaces the instance).
  # The DB lives on the separate data volume, so a replacement is non-destructive.
  user_data_replace_on_change = true

  root_block_device {
    volume_size = var.root_volume_size
    volume_type = "gp3"
    encrypted   = true
  }

  metadata_options {
    http_tokens   = "required" # IMDSv2 only
    http_endpoint = "enabled"
  }

  tags = { Name = "${var.project_name}" }
}

###############################################################################
# Persistent data volume (SQLite DB) — independent of the instance lifecycle
###############################################################################

resource "aws_ebs_volume" "data" {
  availability_zone = aws_instance.this.availability_zone
  size              = var.data_volume_size
  type              = "gp3"
  encrypted         = true

  tags = { Name = "${var.project_name}-data" }

  # Guard against accidental destroy taking the DB with it.
  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf" # appears as an unpartitioned NVMe disk; auto-detected on the box
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.this.id

  # Don't try to detach a busy volume on instance replacement.
  stop_instance_before_detaching = true
}

###############################################################################
# Stable public IP
###############################################################################

resource "aws_eip" "this" {
  instance = aws_instance.this.id
  domain   = "vpc"

  tags = { Name = "${var.project_name}-eip" }
}
