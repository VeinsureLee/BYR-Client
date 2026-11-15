import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/bbs_banner.dart';
import 'package:byr_client/shared/data/models/welcome.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Welcome/banner endpoints (`_WelcomeService`).
class WelcomeApi extends BaseApi {
  WelcomeApi(super.client);

  Future<Result<BBSBanner>> getBanner() =>
      guard(() => client.get('/banner.json'), BBSBanner.fromJson);

  /// Public endpoint: the original sends no `oauth_token`.
  Future<Result<Welcome>> getWelcome() => guard(
    () => client.get('/welcomeimg.json', auth: false),
    Welcome.fromJson,
  );
}
