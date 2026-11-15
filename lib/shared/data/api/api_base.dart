import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/api/api_client.dart';

/// Small helper shared by all service wrappers.
abstract class BaseApi {
  const BaseApi(this.client);

  final ApiClient client;

  Future<Result<T>> guard<T>(
    Future<Map<String, dynamic>> Function() request,
    T Function(Map<String, dynamic>) parse,
  ) async {
    try {
      final json = await request();
      if (json.isEmpty) {
        return const Result.failure('服务器未返回数据');
      }
      return Result.success(parse(json));
    } on ApiException catch (error) {
      return Result.failure(error.message);
    } catch (error) {
      return Result.failure(error.toString());
    }
  }
}
