import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/black_list.dart';
import 'package:byr_client/shared/data/models/status.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Blacklist endpoints (`_BlackListService`).
class BlackListApi extends BaseApi {
  BlackListApi(super.client);

  Future<Result<BlackList>> getBlackList({int page = 1}) => guard(
    () => client.get('/blacklist/list.json', query: {'page': '$page'}),
    BlackList.fromJson,
  );

  Future<Result<Status>> addBlackList(String id) => guard(
    () => client.postForm('/blacklist/add.json', form: {'id': id}),
    Status.fromJson,
  );

  Future<Result<Status>> deleteBlackList(String id) => guard(
    () => client.postForm('/blacklist/delete.json', form: {'id': id}),
    Status.fromJson,
  );
}
