import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Like/downvote status (`remote/model/like.dart`).
class Like extends ApiEnvelope {
  final String? status;
  final int? likeSum;
  final int? votedownSum;

  const Like({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.status,
    this.likeSum,
    this.votedownSum,
  });

  factory Like.fromJson(Map<String, dynamic> json) => Like(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    status: asString(json['status']),
    likeSum: asInt(json['like_sum']),
    votedownSum: asInt(json['votedown_sum']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'status': status,
    'like_sum': likeSum,
    'votedown_sum': votedownSum,
  };
}
