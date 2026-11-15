import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// One uploaded file. Fields recovered from `remote/model/attachment.dart`.
class FileElement {
  final String? name;
  final String? url;
  final String? size;
  final String? thumbnailSmall;
  final String? thumbnailMiddle;

  const FileElement({
    this.name,
    this.url,
    this.size,
    this.thumbnailSmall,
    this.thumbnailMiddle,
  });

  factory FileElement.fromJson(Map<String, dynamic> json) => FileElement(
    name: asString(json['name']),
    url: asString(json['url']),
    size: asString(json['size']),
    thumbnailSmall: asString(json['thumbnail_small']),
    thumbnailMiddle: asString(json['thumbnail_middle']),
  );

  Map<String, dynamic> toJson() => {
    'name': name,
    'url': url,
    'size': size,
    'thumbnail_small': thumbnailSmall,
    'thumbnail_middle': thumbnailMiddle,
  };
}

class Attachment extends ApiEnvelope {
  final List<FileElement> files;
  final String? remainSpace;
  final int? remainCount;

  const Attachment({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.files = const [],
    this.remainSpace,
    this.remainCount,
  });

  factory Attachment.fromJson(Map<String, dynamic> json) => Attachment(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    files: parseList(json['file'], FileElement.fromJson),
    remainSpace: asString(json['remain_space']),
    remainCount: asInt(json['remain_count']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'file': files.map((e) => e.toJson()).toList(),
    'remain_space': remainSpace,
    'remain_count': remainCount,
  };
}
