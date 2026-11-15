import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// One version entry (`Version` in `remote/model/version.dart`).
class Version {
  final String? version;
  final String? changelog;
  final String? url;

  const Version({this.version, this.changelog, this.url});

  factory Version.fromJson(Map<String, dynamic> json) => Version(
    version: asString(json['version']),
    changelog: asString(json['changelog']),
    url: asString(json['url']),
  );

  Map<String, dynamic> toJson() => {
    'version': version,
    'changelog': changelog,
    'url': url,
  };
}

/// Version list response (`AppVersion` in `remote/model/version.dart`).
class AppVersion extends ApiEnvelope {
  final List<Version> versions;

  const AppVersion({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.versions = const [],
  });

  factory AppVersion.fromJson(Map<String, dynamic> json) => AppVersion(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    versions: parseList(json['versions'], Version.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'versions': versions.map((e) => e.toJson()).toList(),
  };
}
