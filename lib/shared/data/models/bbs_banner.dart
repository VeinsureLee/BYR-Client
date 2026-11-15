import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// One banner (`BannerElement` in `remote/model/bbs_banner.dart`).
class BannerElement {
  final String? imageUrl;
  final String? intro;
  final String? url;

  const BannerElement({this.imageUrl, this.intro, this.url});

  factory BannerElement.fromJson(Map<String, dynamic> json) => BannerElement(
    imageUrl: asString(json['image_url']),
    intro: asString(json['intro']),
    url: asString(json['url']),
  );

  Map<String, dynamic> toJson() => {
    'image_url': imageUrl,
    'intro': intro,
    'url': url,
  };
}

/// Banner list (`remote/model/bbs_banner.dart`).
class BBSBanner extends ApiEnvelope {
  final int? count;
  final List<BannerElement> banners;

  const BBSBanner({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.count,
    this.banners = const [],
  });

  factory BBSBanner.fromJson(Map<String, dynamic> json) => BBSBanner(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    count: asInt(json['count']),
    banners: parseList(json['banners'], BannerElement.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'count': count,
    'banners': banners.map((e) => e.toJson()).toList(),
  };
}
