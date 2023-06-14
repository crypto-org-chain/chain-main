local config = import 'default.jsonnet';

config {
  chaintest+: {
    validators: [super.validators[0] {
      'app-config':: super['app-config'],
    }] + super.validators[1:],
  },
}
