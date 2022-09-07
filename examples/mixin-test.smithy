@mixin
@pattern("[a-zA-Z0-1]*")
string AlphaNumericMixin

@length(min: 8, max: 32)
string Username with [AlphaNumericMixin]

@mixin
structure GenericError {
  @required
  message: String
}

structure MyError with [GenericError] {
  description: String
}
