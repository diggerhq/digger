table "sessions" {
  schema = schema.public
  column "id" {
    null = false
    type = text
  }
  column "data" {
    null = true
    type = text
  }
  column "created_at" {
    null = true
    type = timestamptz
  }
  column "updated_at" {
    null = true
    type = timestamptz
  }
  column "expires_at" {
    null = true
    type = timestamptz
  }
  primary_key {
    columns = [column.id]
  }
}
schema "public" {
  comment = "standard public schema"
}
