import 'package:dio/dio.dart';

import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/article.dart';
import 'package:byr_client/shared/data/models/attachment.dart';
import 'package:byr_client/shared/data/models/collection.dart';
import 'package:byr_client/shared/data/models/favor_post.dart';
import 'package:byr_client/shared/data/models/like.dart';
import 'package:byr_client/shared/data/models/recommend_board.dart';
import 'package:byr_client/shared/data/models/thread.dart';
import 'package:byr_client/shared/data/models/top_ten.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Forum endpoints. Paths and return types recovered from `_ForumService`.
class ForumApi extends BaseApi {
  ForumApi(super.client);

  Future<Result<Article>> getArticle({
    required String board,
    required String id,
  }) => guard(() => client.get('/article/$board/$id.json'), Article.fromJson);

  Future<Result<Article>> deleteArticle({
    required String board,
    required String id,
  }) => guard(
    () => client.postForm('/article/$board/delete/$id.json'),
    Article.fromJson,
  );

  Future<Result<Article>> updateArticle({
    required String board,
    required String id,
    required String content,
    String? title,
  }) => guard(
    () => client.postForm(
      '/article/$board/update/$id.json',
      form: {'title': ?title, 'content': content},
    ),
    Article.fromJson,
  );

  Future<Result<Article>> postArticle({
    required String board,
    required String title,
    required String content,
    String? reid,
    bool anonymous = false,
    bool signature = false,
    bool email = false,
    bool outgo = false,
  }) => guard(
    () => client.postForm(
      '/article/$board/post.json',
      form: {
        'title': title,
        'content': content,
        // The official client removes null optional fields from the form.
        // Sending an empty `reid` makes the server treat a new thread as a
        // malformed reply ("reid 不正确"). Only include it for real replies.
        if (reid != null && reid.trim().isNotEmpty) 'reid': reid,
        'signature': signature ? '1' : '0',
        'email': email ? '1' : '0',
        'anonymous': anonymous ? '1' : '0',
        'outgo': outgo ? '1' : '0',
      },
    ),
    Article.fromJson,
  );

  Future<Result<Like>> likeArticle({
    required String board,
    required String id,
  }) => guard(() => client.get('/article/$board/like/$id.json'), Like.fromJson);

  Future<Result<Like>> voteDownArticle({
    required String board,
    required String id,
  }) => guard(
    () => client.get('/article/$board/votedown/$id.json'),
    Like.fromJson,
  );

  Future<Result<Thread>> getThread({
    required String board,
    required String id,
    int page = 1,
    int count = 50,
  }) => guard(
    () => client.get(
      '/threads/$board/$id.json',
      query: {'page': '$page', 'count': '$count'},
    ),
    Thread.fromJson,
  );

  Future<Result<Thread>> getThreadByAuthor({
    required String board,
    required String author,
    int page = 1,
    int count = 50,
  }) => guard(
    () => client.get(
      '/threads/$board/$author.json',
      query: {'page': '$page', 'count': '$count'},
    ),
    Thread.fromJson,
  );

  Future<Result<Attachment>> getAttachment({
    required String board,
    required String id,
  }) => guard(
    () => client.get('/attachment/$board/$id.json'),
    Attachment.fromJson,
  );

  /// Multipart upload. The exact multipart field name is inferred and still
  /// needs runtime verification against the original app.
  Future<Result<Attachment>> addAttachment({
    required String board,
    required String id,
    required String filePath,
    String fieldName = 'file',
  }) async {
    final form = FormData();
    form.files.add(MapEntry(fieldName, await MultipartFile.fromFile(filePath)));
    return guard(
      () => client.postMultipart('/attachment/$board/add/$id.json', form: form),
      Attachment.fromJson,
    );
  }

  Future<Result<Attachment>> deleteAttachment({
    required String board,
    required String id,
  }) => guard(
    () => client.postForm('/attachment/$board/delete/$id.json'),
    Attachment.fromJson,
  );

  Future<Result<Article>> addCollection({
    required String board,
    required String id,
  }) => guard(
    () => client.postForm(
      '/collection/add.json',
      form: {'board': board, 'id': id},
    ),
    Article.fromJson,
  );

  Future<Result<Article>> deleteCollection({
    required String board,
    required String id,
  }) => guard(
    () => client.postForm(
      '/collection/delete.json',
      form: {'board': board, 'id': id},
    ),
    Article.fromJson,
  );

  Future<Result<Collection>> getCollection({
    String? id,
    int page = 1,
    int count = 50,
  }) => guard(
    () => client.get(
      '/collection.json',
      query: {'id': ?id, 'page': '$page', 'count': '$count'},
    ),
    Collection.fromJson,
  );

  Future<Result<FavorPost>> getFavorPost({int page = 1, int count = 50}) =>
      guard(
        () => client.get(
          '/favorpost.json',
          query: {'page': '$page', 'count': '$count'},
        ),
        FavorPost.fromJson,
      );

  Future<Result<RecommendBoard>> getRecommendBoard() => guard(
    () => client.get('/widget/recommend.json'),
    RecommendBoard.fromJson,
  );

  Future<Result<TopTen>> getTopTen() =>
      guard(() => client.get('/widget/topten.json'), TopTen.fromJson);
}
