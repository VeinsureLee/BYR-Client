import 'package:byr_client/shared/core/constants.dart';
import 'package:byr_client/shared/storage/session.dart';
import 'package:byr_client/shared/data/api/api_client.dart';
import 'package:byr_client/shared/data/api/auth_api.dart';
import 'package:byr_client/shared/data/api/bet_api.dart';
import 'package:byr_client/shared/data/api/blacklist_api.dart';
import 'package:byr_client/shared/data/api/board_api.dart';
import 'package:byr_client/shared/data/api/crypto_api.dart';
import 'package:byr_client/shared/data/api/favorite_api.dart';
import 'package:byr_client/shared/data/api/forum_api.dart';
import 'package:byr_client/shared/data/api/mail_api.dart';
import 'package:byr_client/shared/data/api/refer_api.dart';
import 'package:byr_client/shared/data/api/search_api.dart';
import 'package:byr_client/shared/data/api/section_api.dart';
import 'package:byr_client/shared/data/api/update_api.dart';
import 'package:byr_client/shared/data/api/user_api.dart';
import 'package:byr_client/shared/data/api/vote_api.dart';
import 'package:byr_client/shared/data/api/welcome_api.dart';

/// Facade wiring every service, mirroring `Network` in the original app.
class ByrApi {
  ByrApi({required this.session, ApiClient? client})
    : client = client ?? ApiClient();

  final ApiClient client;
  final Session session;

  late final AuthApi auth = AuthApi(client);
  late final ForumApi forum = ForumApi(client);
  late final BoardApi board = BoardApi(client);
  late final SectionApi section = SectionApi(client);
  late final UserApi user = UserApi(client);
  late final SearchApi search = SearchApi(client);
  late final MailApi mail = MailApi(client);
  late final FavoriteApi favorite = FavoriteApi(client);
  late final BlackListApi blackList = BlackListApi(client);
  late final VoteApi vote = VoteApi(client);
  late final BetApi bet = BetApi(client);
  late final ReferApi refer = ReferApi(client);
  late final WelcomeApi welcome = WelcomeApi(client);
  late final UpdateApi update = UpdateApi(client);
  late final CryptoApi crypto = CryptoApi(client);

  /// Restores the persisted token and applies it to the HTTP client.
  Future<bool> restore() async {
    await session.load();
    Constants.useIPv6 = session.useIPv6;
    if (session.isLoggedIn) {
      client.token = session.token;
      if (session.hasCookies) {
        // Legacy sessions may contain nForum cookies whose names (`[`/`]`)
        // are rejected by dart:io's cookie serializer. Never let that block
        // startup.
        try {
          await client.restoreCookies(session.cookies);
        } catch (_) {
          // Ignore stale cookie data.
        }
      }
    } else if (session.hasCookies) {
      // Cookie-only sessions cannot access `/open`; drop them so the login
      // screen is shown instead of an empty workspace.
      await session.saveCookies(const []);
    }
    return session.isAuthenticated;
  }

  /// Applies the IPv6 mirror toggle (`openIPv6` in the original settings).
  void setUseIPv6(bool value) {
    Constants.useIPv6 = value;
  }
}
