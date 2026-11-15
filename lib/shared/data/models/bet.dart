import 'package:byr_client/shared/data/models/json_utils.dart';
import 'package:byr_client/shared/data/models/request.dart';

/// User's own bet (`Mybet` in `remote/model/bet.dart`).
class MyBet {
  final String? uid;
  final String? biid;
  final int? bonus;

  const MyBet({this.uid, this.biid, this.bonus});

  factory MyBet.fromJson(Map<String, dynamic> json) => MyBet(
    uid: asString(json['uid']),
    biid: asString(json['biid']),
    bonus: asInt(json['bonus']),
  );

  Map<String, dynamic> toJson() => {'uid': uid, 'biid': biid, 'bonus': bonus};
}

/// One bet item (`BItem` in `remote/model/bet.dart`).
class BetItem {
  final String? biid;
  final String? label;
  final double? percent;
  final int? num;

  const BetItem({this.biid, this.label, this.percent, this.num});

  factory BetItem.fromJson(Map<String, dynamic> json) => BetItem(
    biid: asString(json['biid']),
    label: asString(json['label']),
    percent: asDouble(json['percent']),
    num: asInt(json['num']),
  );

  Map<String, dynamic> toJson() => {
    'biid': biid,
    'label': label,
    'percent': percent,
    'num': num,
  };
}

/// Bet (`remote/model/bet.dart`).
class Bet extends ApiEnvelope {
  final String? bid;
  final String? cate;
  final String? title;
  final String? desc;
  final String? start;
  final String? end;
  final int? num;
  final int? limit;
  final int? discount;
  final int? score;
  final String? aid;
  final String? result;
  final List<BetItem> items;
  final MyBet? myBet;

  const Bet({
    super.request,
    super.code,
    super.msg,
    super.error,
    this.bid,
    this.cate,
    this.title,
    this.desc,
    this.start,
    this.end,
    this.num,
    this.limit,
    this.discount,
    this.score,
    this.aid,
    this.result,
    this.items = const [],
    this.myBet,
  });

  factory Bet.fromJson(Map<String, dynamic> json) => Bet(
    request: asString(json['request']),
    code: asString(json['code']),
    msg: asString(json['msg']),
    error: asString(json['error']),
    bid: asString(json['bid']),
    cate: asString(json['cate']),
    title: asString(json['title']),
    desc: asString(json['desc']),
    start: asString(json['start']),
    end: asString(json['end']),
    num: asInt(json['num']),
    limit: asInt(json['limit']),
    discount: asInt(json['discount']),
    score: asInt(json['score']),
    aid: asString(json['aid']),
    result: asString(json['result']),
    items: parseList(json['bitems'], BetItem.fromJson),
    myBet: json['mybet'] is Map ? MyBet.fromJson(asMap(json['mybet'])) : null,
  );

  @override
  Map<String, dynamic> toJson() => {
    ...super.toJson(),
    'bid': bid,
    'cate': cate,
    'title': title,
    'desc': desc,
    'start': start,
    'end': end,
    'num': num,
    'limit': limit,
    'discount': discount,
    'score': score,
    'aid': aid,
    'result': result,
    'bitems': items.map((e) => e.toJson()).toList(),
    'mybet': myBet?.toJson(),
  };
}
