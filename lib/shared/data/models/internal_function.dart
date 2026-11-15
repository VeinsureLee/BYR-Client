/// Experimental response (`remote/model/internal_function.dart`).
///
/// The original model only declares a raw `data` field.
class InternalFunction {
  final dynamic data;

  const InternalFunction({this.data});

  factory InternalFunction.fromJson(Map<String, dynamic> json) =>
      InternalFunction(data: json['data']);

  Map<String, dynamic> toJson() => {'data': data};
}
