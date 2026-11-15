import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/version.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Update endpoints (`_UpdateService`).
class UpdateApi extends BaseApi {
  UpdateApi(super.client);

  Future<Result<AppVersion>> getReleaseVersion() => guard(
    () => client.get('/app/release/version.json', auth: false),
    AppVersion.fromJson,
  );

  Future<Result<AppVersion>> getBetaVersion() => guard(
    () => client.get('/app/beta/version.json', auth: false),
    AppVersion.fromJson,
  );
}
