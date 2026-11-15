import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/result.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/models/mail_box.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/post_content.dart';
import 'package:byr_client/features/mail/mail_compose_screen.dart';

class MailScreen extends StatefulWidget {
  const MailScreen({super.key, this.box = 'inbox', this.embedded = false});

  final String box;

  /// When embedded in the "消息" tab, the parent supplies the AppBar.
  final bool embedded;

  @override
  State<MailScreen> createState() => _MailScreenState();
}

class _MailScreenState extends State<MailScreen> {
  late String _box = widget.box;
  Future<Result<MailBox>>? _future;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    final future = AppScope.of(context).api.mail.getMailBox(box: _box);
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
                DropdownButton<String>(
                  value: _box,
                  underline: const SizedBox.shrink(),
                  items: const [
                    DropdownMenuItem(value: 'inbox', child: Text('收件箱')),
                    DropdownMenuItem(value: 'outbox', child: Text('发件箱')),
                  ],
                  onChanged: (value) {
                    if (value == null || value == _box) return;
                    setState(() => _box = value);
                    _load();
                  },
                ),
                const Spacer(),
                TextButton.icon(
                  onPressed: _compose,
                  icon: const Icon(Icons.edit_outlined, size: 18),
                  label: const Text('写邮件'),
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
        title: const Text('信箱'),
        actions: [
          IconButton(
            tooltip: '写邮件',
            onPressed: _compose,
            icon: const Icon(Icons.edit_outlined),
          ),
          DropdownButton<String>(
            value: _box,
            underline: const SizedBox.shrink(),
            items: const [
              DropdownMenuItem(value: 'inbox', child: Text('收件箱')),
              DropdownMenuItem(value: 'outbox', child: Text('发件箱')),
            ],
            onChanged: (value) {
              if (value == null || value == _box) return;
              setState(() => _box = value);
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
    return FutureBuilder<Result<MailBox>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const LoadingView();
        }
        return ResultView<MailBox>(
          result: snapshot.data,
          onRetry: _load,
          builder: (data) {
            if (data.mails.isEmpty) {
              return const EmptyView(message: '信箱为空');
            }
            return RefreshIndicator(
              onRefresh: () async => _load(),
              child: ListView.separated(
                itemCount: data.mails.length,
                separatorBuilder: (_, _) => const Divider(height: 1),
                itemBuilder: (context, index) {
                  final mail = data.mails[index];
                  final time = formatTimestamp(mail.postTime);
                  return ListTile(
                    leading: Icon(
                      mail.isRead == true
                          ? Icons.mail_outline
                          : Icons.mark_email_unread,
                    ),
                    title: Text(
                      mail.title ?? '无标题',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: Text(
                      '${displayUserName(mail.user, fallback: '')}'
                      '${time.isEmpty ? '' : ' · $time'}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    onTap: mail.index == null
                        ? null
                        : () => _openMail(mail.index!),
                  );
                },
              ),
            );
          },
        );
      },
    );
  }

  Future<void> _compose() async {
    final sent = await Navigator.of(
      context,
    ).push<bool>(MaterialPageRoute(builder: (_) => const MailComposeScreen()));
    if (sent == true) _load();
  }

  Future<void> _openMail(String index) async {
    final result = await AppScope.of(context).api.mail
        .getIndexMail(box: _box, id: index);
    if (!mounted) return;
    if (result.isFailure) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '打开失败')));
      return;
    }
    final changed = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => MailDetailScreen(
          mail: result.data as Mail,
          box: _box,
          index: index,
        ),
      ),
    );
    if (changed == true) _load();
  }
}

class MailDetailScreen extends StatefulWidget {
  const MailDetailScreen({
    super.key,
    required this.mail,
    required this.box,
    this.index,
  });

  final Mail mail;
  final String box;
  final String? index;

  @override
  State<MailDetailScreen> createState() => _MailDetailScreenState();
}

class _MailDetailScreenState extends State<MailDetailScreen> {
  Mail get mail => widget.mail;

  Future<void> _reply() async {
    await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => MailComposeScreen(
          mode: MailComposeMode.reply,
          box: widget.box,
          mail: mail,
          mailId: widget.index,
        ),
      ),
    );
  }

  Future<void> _forward() async {
    await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => MailComposeScreen(
          mode: MailComposeMode.forward,
          box: widget.box,
          mail: mail,
          mailId: widget.index,
        ),
      ),
    );
  }

  Future<void> _delete() async {
    final id = mail.index ?? widget.index;
    if (id == null) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('删除邮件'),
        content: const Text('确定删除这封邮件？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    final result = await AppScope.of(context).api.mail
        .deleteMail(box: widget.box, id: id);
    if (!mounted) return;
    if (result.isFailure) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '删除失败')));
      return;
    }
    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(mail.title ?? '邮件'),
        actions: [
          IconButton(
            tooltip: '回复',
            onPressed: _reply,
            icon: const Icon(Icons.reply),
          ),
          IconButton(
            tooltip: '转发',
            onPressed: _forward,
            icon: const Icon(Icons.forward),
          ),
          IconButton(
            tooltip: '删除',
            onPressed: _delete,
            icon: const Icon(Icons.delete_outline),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(
            mail.title ?? '无标题',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            '${displayUserName(mail.user, fallback: '')}'
            '${formatTimestamp(mail.postTime).isEmpty ? '' : ' · ${formatTimestamp(mail.postTime)}'}',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const Divider(height: 24),
          PostContent(
            content: mail.content,
            attachments: mail.attachments,
            token: AppScope.of(context).api.client.token,
          ),
        ],
      ),
    );
  }
}
