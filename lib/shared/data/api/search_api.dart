import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/search.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Search endpoints (`_SearchService`).
class SearchApi extends BaseApi {
  SearchApi(super.client);

  /// Query shape recovered from `_SearchService::searchArticle`.
  Future<Result<SearchArticle>> searchArticle({
    String board = '',
    String title1 = '',
    String title2 = '',
    String titlen = '',
    String author = '',
    String day = '',
    String m = '',
    String a = '',
    String o = '1',
    int count = 50,
    int page = 1,
  }) => guard(
    () => client.get(
      '/search/article.json',
      query: {
        'board': board,
        'title1': title1,
        'title2': title2,
        'titlen': titlen,
        'author': author,
        'day': day,
        'm': m,
        'a': a,
        'o': o,
        'count': '$count',
        'page': '$page',
      },
    ),
    SearchArticle.fromJson,
  );

  Future<Result<SearchBoard>> searchBoard({required String board}) => guard(
    () => client.get('/search/board.json', query: {'board': board}),
    SearchBoard.fromJson,
  );
}
