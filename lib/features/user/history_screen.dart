import 'package:byr_client/app/app_scope.dart';
import 'package:material_ui/material_ui.dart';

import 'package:byr_client/shared/storage/history_store.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/features/thread/thread_screen.dart';

/// Local browsing history of opened threads.
class HistoryScreen extends StatefulWidget {
  const HistoryScreen({super.key});

  @override
  State<HistoryScreen> createState() => _HistoryScreenState();
}

class _HistoryScreenState extends State<HistoryScreen> {
  List<HistoryEntry>? _entries;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final entries = await AppScope.of(context).history.load();
    if (!mounted) return;
    setState(() => _entries = entries);
  }

  Future<void> _clear() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('清空浏览历史'),
        content: const Text('确定清空全部本地浏览记录？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('清空'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    await AppScope.of(context).history.clear();
    await _load();
  }

  String _formatTime(int ms) {
    if (ms <= 0) return '';
    final time = DateTime.fromMillisecondsSinceEpoch(ms).toLocal();
    String two(int value) => value.toString().padLeft(2, '0');
    return '${time.year}-${two(time.month)}-${two(time.day)} '
        '${two(time.hour)}:${two(time.minute)}';
  }

  @override
  Widget build(BuildContext context) {
    final entries = _entries;
    return Scaffold(
      appBar: AppBar(
        title: const Text('浏览历史'),
        actions: [
          IconButton(
            tooltip: '清空',
            onPressed: entries == null || entries.isEmpty ? null : _clear,
            icon: const Icon(Icons.delete_sweep_outlined),
          ),
        ],
      ),
      body: entries == null
          ? const LoadingView()
          : entries.isEmpty
          ? const EmptyView(message: '还没有浏览记录')
          : ListView.separated(
              itemCount: entries.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, index) {
                final entry = entries[index];
                return ListTile(
                  leading: const Icon(Icons.history),
                  title: Text(
                    entry.title.isEmpty ? '无标题' : entry.title,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  subtitle: Text(
                    '${entry.board} · ${_formatTime(entry.visitedAt)}',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  onTap: () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => ThreadScreen(
                        boardName: entry.board,
                        threadId: entry.id,
                      ),
                    ),
                  ),
                );
              },
            ),
    );
  }
}
