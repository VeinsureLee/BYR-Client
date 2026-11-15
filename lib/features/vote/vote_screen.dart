import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/data/models/vote.dart';
import 'package:byr_client/shared/widgets/common.dart';

class VoteScreen extends StatefulWidget {
  const VoteScreen({super.key, required this.aid});

  /// Article id the vote is attached to.
  final String aid;

  @override
  State<VoteScreen> createState() => _VoteScreenState();
}

class _VoteScreenState extends State<VoteScreen> {
  Future<Result<Vote>>? _future;
  String? _selected;
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.vote.getVote(aid: widget.aid);
    setState(() {
      _future = future;
    });
  }

  Future<void> _submit() async {
    final choice = _selected;
    if (choice == null) return;
    setState(() => _submitting = true);
    final result = await AppScope.of(context).api.vote
        .voteOn(aid: widget.aid, vote: choice);
    if (!mounted) return;
    setState(() => _submitting = false);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(result.isSuccess ? '投票成功' : (result.message ?? '投票失败')),
      ),
    );
    if (result.isSuccess) _load();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('投票')),
      body: FutureBuilder<Result<Vote>>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const LoadingView();
          }
          return ResultView<Vote>(
            result: snapshot.data,
            onRetry: _load,
            builder: (vote) {
              return ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  Text(
                    vote.title ?? '投票',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  if (vote.end != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 4, bottom: 12),
                      child: Text(
                        '截止：${vote.end}',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ),
                  for (final option in vote.options)
                    RadioListTile<String>(
                      value: option.viid ?? option.label ?? '',
                      groupValue: _selected,
                      title: Text(option.label ?? '选项'),
                      subtitle: option.num == null
                          ? null
                          : Text('${option.num} 票'),
                      onChanged: vote.isEnd == true
                          ? null
                          : (value) => setState(() => _selected = value),
                    ),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: _submitting || vote.isEnd == true
                        ? null
                        : _submit,
                    child: _submitting
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Text(vote.isEnd == true ? '投票已结束' : '提交投票'),
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
