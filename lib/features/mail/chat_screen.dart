import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/core/time_format.dart';
import 'package:byr_client/shared/data/api/mail_api.dart';
import 'package:byr_client/shared/data/models/mail_box.dart';
import 'package:byr_client/shared/data/models/user.dart';
import 'package:byr_client/shared/widgets/common.dart';
import 'package:byr_client/shared/widgets/post_content.dart';
import 'package:byr_client/features/mail/mail_compose_screen.dart';

class _Contact {
  _Contact({
    required this.id,
    required this.name,
    required this.user,
    required this.lastTime,
    required this.preview,
  });

  final String id;
  final String name;
  final User? user;
  final int lastTime;
  final String preview;
}

class _MailEntry {
  const _MailEntry({required this.mail, required this.mine});

  final Mail mail;
  final bool mine;
}

class _ChatMessage {
  const _ChatMessage({
    required this.mail,
    required this.mine,
    required this.content,
  });

  final Mail mail;
  final bool mine;
  final String content;

  int get time => int.tryParse(mail.postTime ?? '') ?? 0;
}

/// Aggregate inbox + outbox mails by contact.
class ChatListScreen extends StatefulWidget {
  const ChatListScreen({super.key, this.embedded = false});

  /// When embedded in the "消息" tab, the parent supplies the AppBar.
  final bool embedded;

  @override
  State<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends State<ChatListScreen> {
  List<_Contact>? _contacts;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _error = null;
      _contacts = null;
    });
    final api = AppScope.of(context).api;
    final results = await Future.wait([
      api.mail.getMailBox(box: 'inbox', count: 100),
      api.mail.getMailBox(box: 'outbox', count: 100),
    ]);
    if (!mounted) return;
    final byContact = <String, _Contact>{};
    for (final result in results) {
      if (result.isFailure || result.data == null) continue;
      for (final mail in result.data!.mails) {
        final id = mail.user?.id;
        if (id == null || id.isEmpty) continue;
        final time = int.tryParse(mail.postTime ?? '') ?? 0;
        final existing = byContact[id];
        if (existing == null || time > existing.lastTime) {
          byContact[id] = _Contact(
            id: id,
            name: displayUserName(mail.user, fallback: id),
            user: mail.user,
            lastTime: time,
            preview: (mail.title ?? '').trim(),
          );
        }
      }
    }
    if (byContact.isEmpty && results.every((r) => r.isFailure)) {
      setState(() => _error = results.first.message ?? '加载私信失败');
      return;
    }
    final contacts = byContact.values.toList()
      ..sort((a, b) => b.lastTime.compareTo(a.lastTime));
    setState(() => _contacts = contacts);
  }

  @override
  Widget build(BuildContext context) {
    final body = _buildBody();
    if (widget.embedded) return body;
    return Scaffold(
      appBar: AppBar(
        title: const Text('私信'),
        actions: [
          IconButton(
            tooltip: '刷新',
            onPressed: _load,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: body,
    );
  }

  Widget _buildBody() {
    return _error != null
        ? ErrorView(message: _error!, onRetry: _load)
        : _contacts == null
        ? const LoadingView()
        : _contacts!.isEmpty
        ? const EmptyView(message: '还没有私信')
        : RefreshIndicator(
            onRefresh: _load,
            child: ListView.separated(
              itemCount: _contacts!.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (context, index) {
                final contact = _contacts![index];
                return ListTile(
                  leading: const CircleAvatar(child: Icon(Icons.person)),
                  title: Text(contact.name),
                  subtitle: Text(
                    contact.preview,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  trailing: contact.lastTime > 0
                      ? Text(formatUnixSeconds(contact.lastTime))
                      : null,
                  onTap: () => Navigator.of(context).push(
                    MaterialPageRoute(
                      builder: (_) => ChatScreen(
                        contactId: contact.id,
                        contactName: contact.name,
                      ),
                    ),
                  ),
                );
              },
            ),
          );
  }
}

class ChatScreen extends StatefulWidget {
  const ChatScreen({
    super.key,
    required this.contactId,
    required this.contactName,
  });

  final String contactId;
  final String contactName;

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final _controller = TextEditingController();
  List<_ChatMessage>? _messages;
  bool _sending = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final api = AppScope.of(context).api;
    final results = await Future.wait([
      api.mail.getMailBox(box: 'inbox', count: 100),
      api.mail.getMailBox(box: 'outbox', count: 100),
    ]);
    if (!mounted) return;

    final entries = <_MailEntry>[];
    for (var i = 0; i < results.length; i++) {
      final result = results[i];
      if (result.data == null) continue;
      final mine = i == 1;
      for (final mail in result.data!.mails) {
        if (mail.user?.id == widget.contactId) {
          entries.add(_MailEntry(mail: mail, mine: mine));
        }
      }
    }
    entries.sort((a, b) {
      final aTime = int.tryParse(a.mail.postTime ?? '') ?? 0;
      final bTime = int.tryParse(b.mail.postTime ?? '') ?? 0;
      return aTime.compareTo(bTime);
    });

    // The mailbox list endpoint does not include `content`; only
    // `/mail/<box>/<index>.json` does. A chat bubble rendered straight from
    // the list therefore always says "(无内容)". Fetch the recent details;
    // cap the number of requests to keep opening a long conversation cheap.
    const maxMessages = 80;
    final recent = entries.length > maxMessages
        ? entries.sublist(entries.length - maxMessages)
        : entries;
    final details = await _loadDetails(api.mail, recent);
    if (!mounted) return;

    setState(() {
      _messages = details;
      _error = results.every((r) => r.isFailure)
          ? results.first.message ?? '加载失败'
          : null;
    });
  }

  Future<List<_ChatMessage>> _loadDetails(
    MailApi api,
    List<_MailEntry> entries,
  ) async {
    final details = List<Mail?>.filled(entries.length, null);
    var next = 0;

    Future<void> worker() async {
      while (true) {
        final index = next++;
        if (index >= entries.length) return;
        final entry = entries[index];
        final mailId = entry.mail.index;
        if (mailId == null || mailId.isEmpty) continue;
        final result = await api.getIndexMail(
          box: entry.mine ? 'outbox' : 'inbox',
          id: mailId,
        );
        if (result.data != null) details[index] = result.data;
      }
    }

    await Future.wait(List.generate(4, (_) => worker()));

    return [
      for (var i = 0; i < entries.length; i++)
        _ChatMessage(
          mail: details[i] ?? entries[i].mail,
          mine: entries[i].mine,
          content: _messageContent(details[i] ?? entries[i].mail),
        ),
    ];
  }

  static String _messageContent(Mail mail) {
    final content = mail.content?.trim() ?? '';
    if (content.isNotEmpty) return content;
    // A mail without a body still has a meaningful subject; showing the
    // title is better than a long conversation full of "(无内容)".
    return mail.title?.trim() ?? '';
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    if (text.isEmpty || _sending) return;
    setState(() => _sending = true);
    final result = await AppScope.of(context).api.mail
        .sendMail(id: widget.contactId, content: text);
    if (!mounted) return;
    setState(() => _sending = false);
    if (result.isFailure) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(result.message ?? '发送失败')));
      return;
    }
    _controller.clear();
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final messages = _messages;
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.contactName),
        actions: [
          IconButton(
            tooltip: '刷新',
            onPressed: _load,
            icon: const Icon(Icons.refresh),
          ),
          IconButton(
            tooltip: '写邮件',
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) =>
                    MailComposeScreen(initialRecipient: widget.contactId),
              ),
            ),
            icon: const Icon(Icons.edit_outlined),
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: _error != null
                ? ErrorView(message: _error!, onRetry: _load)
                : messages == null
                ? const LoadingView()
                : messages.isEmpty
                ? const EmptyView(message: '还没有对话')
                : ListView.builder(
                    padding: const EdgeInsets.all(12),
                    itemCount: messages.length,
                    itemBuilder: (context, index) {
                      final message = messages[index];
                      final align = message.mine
                          ? CrossAxisAlignment.end
                          : CrossAxisAlignment.start;
                      final color = message.mine
                          ? Theme.of(context).colorScheme.primaryContainer
                          : Theme.of(context)
                                .colorScheme
                                .surfaceContainerHighest;
                      return Column(
                        crossAxisAlignment: align,
                        children: [
                          Container(
                            constraints: BoxConstraints(
                              maxWidth: MediaQuery.sizeOf(context).width * 0.72,
                            ),
                            margin: const EdgeInsets.only(bottom: 8),
                            padding: const EdgeInsets.all(10),
                            decoration: BoxDecoration(
                              color: color,
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: PostContent(
                              content: message.content,
                              attachments: message.mail.attachments,
                              token: AppScope.of(context).api.client.token,
                              maxImageWidth: 260,
                            ),
                          ),
                          if (message.time > 0)
                            Padding(
                              padding: const EdgeInsets.only(bottom: 8),
                              child: Text(
                                formatUnixSeconds(message.time),
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            ),
                        ],
                      );
                    },
                  ),
          ),
          SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.all(8),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      textInputAction: TextInputAction.send,
                      onSubmitted: (_) => _send(),
                      decoration: const InputDecoration(
                        hintText: '发送私信…',
                        border: OutlineInputBorder(),
                        isDense: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    onPressed: _sending ? null : _send,
                    icon: _sending
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Icon(Icons.send),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
