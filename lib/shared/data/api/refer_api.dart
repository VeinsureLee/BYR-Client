import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/refer.dart';
import 'package:byr_client/shared/data/models/status.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Refer (mention/reply notification) endpoints (`_ReferService`).
class ReferApi extends BaseApi {
  ReferApi(super.client);

  Future<Result<Refer>> getRefer({
    required String type,
    int page = 1,
    int count = 50,
  }) => guard(
    () => client.get(
      '/refer/$type.json',
      query: {'page': '$page', 'count': '$count'},
    ),
    Refer.fromJson,
  );

  Future<Result<ReferInfo>> getReferInfo({required String type}) =>
      guard(() => client.get('/refer/$type/info.json'), ReferInfo.fromJson);

  Future<Result<Refer>> setReferIndexRead({
    required String type,
    required String index,
  }) => guard(
    () => client.postForm('/refer/$type/setRead/$index.json'),
    Refer.fromJson,
  );

  Future<Result<Status>> setReferAllRead({required String type}) => guard(
    () => client.postForm('/refer/$type/setRead.json'),
    Status.fromJson,
  );
}
