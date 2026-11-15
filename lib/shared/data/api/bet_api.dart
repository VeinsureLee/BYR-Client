import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/bet.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Bet endpoints (`_BetService`).
class BetApi extends BaseApi {
  BetApi(super.client);

  Future<Result<Bet>> getBet({required String bid}) =>
      guard(() => client.get('/bet/$bid.json'), Bet.fromJson);

  Future<Result<Bet>> betOn({
    required String bid,
    required String biid,
    required int score,
  }) => guard(
    () => client.postForm(
      '/bet/$bid.json',
      form: {'biid': biid, 'score': '$score'},
    ),
    Bet.fromJson,
  );
}
