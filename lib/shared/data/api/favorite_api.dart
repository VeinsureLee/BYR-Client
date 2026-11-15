import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/favorite.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Favorite endpoints (`_FavoriteService`).
class FavoriteApi extends BaseApi {
  FavoriteApi(super.client);

  /// Reads one favorite directory. `dir` is `0` for the root directory.
  Future<Result<Favorite>> getFavorite(String dir) =>
      guard(() => client.get('/favorite/$dir.json'), Favorite.fromJson);

  /// Adds a board to the root favorite directory.
  ///
  /// Recovered from `_FavoriteService::addFavorite` (APK): the path is the
  /// fixed `/favorite/add/0.json`; the board name and directory are form
  /// fields (`name`, `dir`). The old `/favorite/add/<board>.json` shape is
  /// rejected by the server with "链接不存在".
  Future<Result<Favorite>> addFavorite(String name, {String dir = '0'}) =>
      guard(
        () => client.postForm(
          '/favorite/add/$dir.json',
          form: {'name': name, 'dir': dir},
        ),
        Favorite.fromJson,
      );

  /// Removes a board from the root favorite directory.
  Future<Result<Favorite>> removeFavorite(String name, {String dir = '0'}) =>
      guard(
        () => client.postForm(
          '/favorite/delete/$dir.json',
          form: {'name': name, 'dir': dir},
        ),
        Favorite.fromJson,
      );
}
