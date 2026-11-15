import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/vote.dart';
import 'package:byr_client/shared/data/api/api_base.dart';

/// Vote endpoints (`_VoteService`).
class VoteApi extends BaseApi {
  VoteApi(super.client);

  Future<Result<Vote>> getVote({required String aid}) =>
      guard(() => client.get('/vote/$aid.json'), Vote.fromJson);

  Future<Result<Vote>> voteOn({required String aid, required String vote}) =>
      guard(
        () => client.postForm('/vote/$aid.json', form: {'vote': vote}),
        Vote.fromJson,
      );

  /// Multi-option vote. The original forwards a map produced by
  /// `VoteMultiplePostBody`; pass the raw form fields here.
  Future<Result<Vote>> voteOnMultiple({
    required String aid,
    required Map<String, dynamic> form,
  }) => guard(
    () => client.postForm('/vote/$aid.json', form: form),
    Vote.fromJson,
  );
}
