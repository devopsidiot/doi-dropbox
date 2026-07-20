data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "lambda_s3" {
  statement {
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.uploads.arn}/*"]
  }
}

resource "aws_iam_role" "lambda_exec" {
  name               = "dropbox-lambda-exec"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy" "lambda_s3" {
  name   = "dropbox-lambda-s3-put"
  role   = aws_iam_role.lambda_exec.id
  policy = data.aws_iam_policy_document.lambda_s3.json
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../lambda/bootstrap" # the compiled Go binary
  output_path = "${path.module}/lambda_function.zip"
}

resource "aws_lambda_function" "presigned_url" {
  function_name = "dropbox-presigned-url"
  role          = aws_iam_role.lambda_exec.arn

  runtime = "provided.al2023"
  handler = "bootstrap"

  architectures = ["arm64"]

  filename         = data.archive_file.lambda_zip.output_path
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256

  timeout     = 10
  memory_size = 128

  environment {
    variables = {
      BUCKET_NAME        = aws_s3_bucket.uploads.id
      URL_EXPIRY_SECONDS = "300"
    }
  }
}
