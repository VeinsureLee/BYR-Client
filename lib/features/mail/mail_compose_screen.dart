import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/shared/data/models/mail_box.dart';

enum MailComposeMode { newMail, reply, forward }

/// Compose a new mail, reply to a mail, or forward it to another user.
class MailComposeScreen extends StatefulWidget {
  const MailComposeScreen({
    super.key,
    this.mode = MailComposeMode.newMail,
    this.box = 'inbox',
    this.mail,
    this.mailId,
    this.initialRecipient,
  });

  final MailComposeMode mode;
  final String box;
  final Mail? mail;
  final String? mailId;
  final String? initialRecipient;

  @override
  State<MailComposeScreen> createState() => _MailComposeScreenState();
}

class _MailComposeScreenState extends State<MailComposeScreen> {
  final _recipientController = TextEditingController();
  late final TextEditingController _titleController = TextEditingController(
    text: widget.mode == MailComposeMode.reply
        ? 'Re: ${widget.mail?.title ?? ''}'
        : '',
  );
  late final TextEditingController _contentController = TextEditingController();
  bool _busy = false;
  String? _error;

  bool get _isForward => widget.mode == MailComposeMode.forward;
  bool get _needsRecipient => widget.mode != MailComposeMode.reply;

  @override
  void initState() {
    super.initState();
    if (widget.mode == MailComposeMode.reply) {
      _recipientController.text = widget.mail?.user?.id ?? '';
    } else if (widget.initialRecipient != null) {
      _recipientController.text = widget.initialRecipient!;
    }
  }

  @override
  void dispose() {
    _recipientController.dispose();
    _titleController.dispose();
    _contentController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final recipient = _recipientController.text.trim();
    final title = _titleController.text.trim();
    final content = _contentController.text.trim();
    if (_needsRecipient && recipient.isEmpty) {
      setState(() => _error = _isForward ? '请输入转发对象' : '请输入收件人');
      return;
    }
    if (!_isForward && content.isEmpty) {
      setState(() => _error = '请输入正文');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
    });

    final api = AppScope.of(context).api;
    final mail = widget.mail;
    if (widget.mode == MailComposeMode.newMail) {
      final result = await api.mail.sendMail(
        id: recipient,
        title: title,
        content: content,
      );
      if (!mounted) return;
      if (result.isFailure) {
        setState(() {
          _busy = false;
          _error = result.message ?? '发送失败';
        });
        return;
      }
    } else if (widget.mode == MailComposeMode.reply) {
      final id = mail?.index ?? widget.mailId;
      if (id == null) {
        setState(() => _error = '无法确定原邮件编号');
        return;
      }
      final result = await api.mail.replyMail(
        box: widget.box,
        id: id,
        title: title,
        content: content,
      );
      if (!mounted) return;
      if (result.isFailure) {
        setState(() {
          _busy = false;
          _error = result.message ?? '回复失败';
        });
        return;
      }
    } else {
      final id = mail?.index ?? widget.mailId;
      if (id == null) {
        setState(() => _error = '无法确定原邮件编号');
        return;
      }
      final result = await api.mail.forwardMail(
        box: widget.box,
        id: id,
        target: recipient,
      );
      if (!mounted) return;
      if (result.isFailure) {
        setState(() {
          _busy = false;
          _error = result.message ?? '转发失败';
        });
        return;
      }
    }

    if (!mounted) return;
    Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    final title = switch (widget.mode) {
      MailComposeMode.newMail => '写邮件',
      MailComposeMode.reply => '回复邮件',
      MailComposeMode.forward => '转发邮件',
    };
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (_needsRecipient)
              TextField(
                controller: _recipientController,
                enabled: !_busy,
                decoration: InputDecoration(
                  labelText: _isForward ? '转发给' : '收件人',
                  hintText: '论坛用户名',
                  border: const OutlineInputBorder(),
                ),
              ),
            if (_needsRecipient) const SizedBox(height: 12),
            if (!_isForward) ...[
              TextField(
                controller: _titleController,
                enabled: !_busy,
                decoration: const InputDecoration(
                  labelText: '标题',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              Expanded(
                child: TextField(
                  controller: _contentController,
                  enabled: !_busy,
                  expands: true,
                  maxLines: null,
                  minLines: null,
                  textAlignVertical: TextAlignVertical.top,
                  decoration: const InputDecoration(
                    labelText: '正文',
                    alignLabelWithHint: true,
                    border: OutlineInputBorder(),
                  ),
                ),
              ),
            ] else
              Expanded(
                child: Center(
                  child: Text(
                    '转发原邮件：${widget.mail?.title ?? ''}',
                    textAlign: TextAlign.center,
                  ),
                ),
              ),
            const SizedBox(height: 12),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                  _error!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              ),
            FilledButton(
              onPressed: _busy ? null : _submit,
              child: _busy
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(_isForward ? '转发' : '发送'),
            ),
          ],
        ),
      ),
    );
  }
}
