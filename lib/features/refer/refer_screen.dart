import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/refer.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/thread/thread_screen.dart';

class ReferScreen extends StatefulWidget {
  const ReferScreen({super.key, this.type = 'reply', this.embedded = false});

  final String type;

  /// When embedded in the "消息" tab, the parent supplies the AppBar.
  final bool embedded;

  @override
  State<ReferScreen> createState() => _ReferScreenState();
}

class _ReferScreenState extends State<ReferScreen> {
  late String _type = widget.type;
  Future<Result<Refer>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.refer.getRefer(type: _type);
    setState(() {
      _future = future;
    });
  }

  @override
  Widget build(BuildContext context) {
    final body = _buildBody();
    if (widget.embedded) {
      return Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
            child: Row(
              children: [
                SegmentedButton<String>(
                  segments: const [
                    ButtonSegment(value: 'reply', label: Text('回复提醒')),
                    ButtonSegment(value: 'at', label: Text('@ 提醒')),
                  ],
                  selected: {_type},
                  onSelectionChanged: (values) {
                    final value = values.first;
                    if (value == _type) return;
                    setState(() => _type = value);
                    _load();
                  },
                ),
              ],
            ),
          ),
          Expanded(child: body),
        ],
      );
    }
    return Scaffold(
      appBar: AppBar(
        title: const Text('提醒'),
        actions: [
          DropdownButton<String>(
            value: _type,
            underline: const SizedBox.shrink(),
            items: const [
              DropdownMenuItem(value: 'reply', child: Text('回复提醒')),
              DropdownMenuItem(value: 'at', child: Text('@ 提醒')),
            ],
            onChanged: (value) {
              if (value == null || value == _type) return;
              setState(() => _type = value);
              _load();
            },
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: body,
    );
  }

  Widget _buildBody() {
    return FutureBuilder<Result<Refer>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const LoadingView();
        }
        return ResultView<Refer>(
          result: snapshot.data,
          onRetry: _load,
          builder: (data) {
            if (data.articles.isEmpty) {
              return const EmptyView(message: '暂无提醒');
            }
            return RefreshIndicator(
              onRefresh: () async => _load(),
              child: ListView.separated(
                itemCount: data.articles.length,
                separatorBuilder: (_, _) => const Divider(height: 1),
                itemBuilder: (context, index) {
                  final item = data.articles[index];
                  final time = formatTimestamp(item.time);
                  return ListTile(
                    leading: Icon(
                      item.isRead == true
                          ? Icons.mark_email_read
                          : Icons.mark_email_unread,
                    ),
                    title: Text(
                      item.title ?? '无标题',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(
                      '${displayUserName(item.user, fallback: '')} · '
                      '${item.boardName ?? ''}'
                      '${time.isEmpty ? '' : ' · $time'}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    onTap: item.boardName == null || item.id == null
                        ? null
                        : () => Navigator.of(context).push(
                            MaterialPageRoute(
                              builder: (_) => ThreadScreen(
                                boardName: item.boardName!,
                                threadId: item.id!,
                              ),
                            ),
                          ),
                  );
                },
              ),
            );
          },
        );
      },
    );
  }
}
