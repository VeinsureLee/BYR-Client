import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Board endpoints (`_BoardService`).
class BoardApi extends BaseApi {
  BoardApi(super.client);

  Future<Result<Board>> getBoard({
    required String board,
    int page = 1,
    int count = 50,
  }) => guard(
    () => client.get(
      '/board/$board.json',
      query: {'page': '$page', 'count': '$count'},
    ),
    Board.fromJson,
  );
}
