# Peering connection
resource "aws_vpc_peering_connection" "infra_stage" {
  vpc_id      = module.vpc.vpc_id
  peer_vpc_id = var.stage_vpc_id
  auto_accept = true

  accepter {
    allow_remote_vpc_dns_resolution = true
  }

  requester {
    allow_remote_vpc_dns_resolution = true
  }

  tags = {
    Requester = "infra"
    Acceptor  = "stage"
  }
}

# Routes
resource "aws_route" "infra" {
  route_table_id            = module.vpc.public_route_table_ids[0]
  destination_cidr_block    = var.stage_vpc_cidr
  vpc_peering_connection_id = aws_vpc_peering_connection.infra_stage.id
}

resource "aws_route" "stage" {
  provider = aws.prod

  route_table_id            = var.stage_vpc_public_route_table_ids[0]
  destination_cidr_block    = module.vpc.vpc_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.infra_stage.id
}
