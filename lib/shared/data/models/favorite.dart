import 'package:byr_client/shared/data/models/board.dart';
import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';
import 'package:byr_client/shared/data/models/section.dart';

/// Favorite tree (`remote/model/favorite.dart`): sub favorites, sections and
/// boards.
class Favorite extends ApiEnvelope {
  final List<Favorite> subFavorites;
  final Section? section;
  final List<Board> boards;

  const Favorite({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.subFavorites = const [],
    this.section,
    this.boards = const [],
  });

  factory Favorite.fromJson(Map<String, dynamic> json) => Favorite(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    subFavorites: parseList(json['sub_favorite'], Favorite.fromJson),
    section: json['section'] is Map
        ? Section.fromJson(asMap(json['section']))
        : null,
    boards: parseList(json['board'], Board.fromJson),
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'sub_favorite': subFavorites.map((e) => e.toJson()).toList(),
    'section': section?.toJson(),
    'board': boards.map((e) => e.toJson()).toList(),
  };
}
