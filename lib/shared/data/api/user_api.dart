import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// User endpoints (`_UserService`).
class UserApi extends BaseApi {
  UserApi(super.client);

  Future<Result<User>> getSelfUserInfo() =>
      guard(() => client.get('/user/getinfo.json'), User.fromJson);

  Future<Result<User>> getUserInfo(String id) =>
      guard(() => client.get('/user/query/$id.json'), User.fromJson);
}
