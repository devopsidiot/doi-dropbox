resource "aws_route53_record" "site" {
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"

  # The apex A record already existed before this stack did — it pointed at an
  # older CloudFront distribution serving the previous site. The provider
  # defaults this to false and fails rather than touch a record it did not
  # create, which is the right default but not what we want here: taking
  # ownership of the record is the whole point of the cutover.
  allow_overwrite = true

  alias {
    name                   = aws_cloudfront_distribution.site.domain_name
    zone_id                = aws_cloudfront_distribution.site.hosted_zone_id
    evaluate_target_health = false
  }
}
