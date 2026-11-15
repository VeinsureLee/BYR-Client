/// Mirrors `package:byr_flutter/remote/network/result.dart`.
enum ResultState { initial, loading, success, failure }

class Result<T> {
  final ResultState state;
  final T? data;
  final String? message;

  const Result._(this.state, this.data, this.message);

  const Result.initial() : this._(ResultState.initial, null, null);
  const Result.loading() : this._(ResultState.loading, null, null);
  const Result.success(T value) : this._(ResultState.success, value, null);
  const Result.failure([String? message])
    : this._(ResultState.failure, null, message);

  bool get isSuccess => state == ResultState.success;
  bool get isFailure => state == ResultState.failure;

  Result<R> map<R>(R Function(T) transform) {
    if (isSuccess) return Result.success(transform(data as T));
    return Result._(state, null, message);
  }

  @override
  String toString() => 'Result($state, data: $data, message: $message)';
}
