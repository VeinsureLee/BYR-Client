import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/bet.dart';
import 'package:byr_client/shared/widgets/common.dart';

class BetScreen extends StatefulWidget {
  const BetScreen({super.key, required this.bid});

  /// Article id the bet is attached to.
  final String bid;

  @override
  State<BetScreen> createState() => _BetScreenState();
}

class _BetScreenState extends State<BetScreen> {
  Future<Result<Bet>>? _future;
  String? _selected;
  final _scoreController = TextEditingController(text: '10');
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _scoreController.dispose();
    super.dispose();
  }

  void _load() {
    final future = AppScope.of(context).api.bet.getBet(bid: widget.bid);
    setState(() {
      _future = future;
    });
  }

  Future<void> _submit() async {
    final biid = _selected;
    final score = int.tryParse(_scoreController.text.trim());
    if (biid == null || score == null || score <= 0) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('请选择选项并输入有效积分')));
      return;
    }
    setState(() => _submitting = true);
    final result = await AppScope.of(context).api.bet
        .betOn(bid: widget.bid, biid: biid, score: score);
    if (!mounted) return;
    setState(() => _submitting = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(result.isSuccess ? '下注成功' : (result.message ?? '下注失败')),
      ),
    );
    if (result.isSuccess) _load();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('竞猜')),
      body: FutureBuilder<Result<Bet>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<Bet>(
            result: snapshot.data,
            onRetry: _load,
            builder: (bet) {
              return ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  Text(
                    bet.title ?? '竞猜',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  if (bet.desc != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Text(bet.desc!),
                    ),
                  const SizedBox(height: 12),
                  for (final item in bet.items)
                    RadioListTile<String>(
                      value: item.biid ?? item.label ?? '',
                      groupValue: _selected,
                      title: Text(item.label ?? '选项'),
                      subtitle: item.percent == null
                          ? null
                          : Text(
                              '${(item.percent! * 100).toStringAsFixed(1)}%',
                            ),
                      onChanged: (value) => setState(() => _selected = value),
                    ),
                  const SizedBox(height: 12),
                  TextField(
                    controller: _scoreController,
                    keyboardType: TextInputType.number,
                    decoration: const InputDecoration(
                      labelText: '下注积分',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  if (bet.myBet != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text('我的下注：${bet.myBet!.bonus ?? 0} 积分'),
                    ),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: _submitting ? null : _submit,
                    child: _submitting
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('下注'),
                  ),
                ],
              );
            },
          );
        },
      ),
    );
  }
}
