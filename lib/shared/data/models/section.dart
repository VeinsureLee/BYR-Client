import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// Section (分区) with its boards. Fields recovered from
/// `remote/model/section.dart`.
class Section extends ApiEnvelope {
  final String? name;
  final String? description;
  final bool? isRoot;
  final String? parent;
  final List<Section> subSections;
  final List<Board> boards;

  const Section({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.name,
    this.description,
    this.isRoot,
    this.parent,
    this.subSections = const [],
    this.boards = const [],
  });

  factory Section.fromJson(Map<String, dynamic> json) => Section(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    name: asString(json['name']),
    description: asString(json['description']),
    isRoot: asBool(json['is_root']),
    parent: asString(json['parent']),
    subSections: parseList(json['sub_section'], Section.fromJson),
    boards: parseList(json['board'], Board.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'name': name,
    'description': description,
    'is_root': isRoot,
    'parent': parent,
    'sub_section': subSections.map((e) => e.toJson()).toList(),
    'board': boards.map((e) => e.toJson()).toList(),
  };
}
