import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Element of the root section list. Fields recovered from
/// `remote/model/root_section.dart` (`SectionElement`).
class SectionElement extends ApiEnvelope {
  final String? name;
  final String? description;

  /// The server field is misspelled `is_rooot` in the original model.
  final bool? isRoot;
  final String? parent;

  const SectionElement({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.name,
    this.description,
    this.isRoot,
    this.parent,
  });

  factory SectionElement.fromJson(Map<String, dynamic> json) => SectionElement(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    name: asString(json['name']),
    description: asString(json['description']),
    isRoot: asBool(json['is_rooot']),
    parent: asString(json['parent']),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'name': name,
    'description': description,
    'is_rooot': isRoot,
    'parent': parent,
  };
}

/// Root section list (`remote/model/root_section.dart`).
class RootSection extends ApiEnvelope {
  final int? sectionCount;
  final List<SectionElement> sections;

  const RootSection({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.sectionCount,
    this.sections = const [],
  });

  factory RootSection.fromJson(Map<String, dynamic> json) => RootSection(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    sectionCount: asInt(json['section_count']),
    sections: parseList(json['section'], SectionElement.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'section_count': sectionCount,
    'section': sections.map((e) => e.toJson()).toList(),
  };
}
