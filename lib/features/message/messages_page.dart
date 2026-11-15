import 'package:material_ui/material_ui.dart';

import 'package:byr_client/features/mail/chat_screen.dart';
import 'package:byr_client/features/mail/mail_screen.dart';
import 'package:byr_client/features/refer/refer_screen.dart';

/// Bottom-navigation "消息" page: private chats, mailbox and mentions/replies.
///
/// The original app's `/main/message` route is a chat/contact list; the mailbox
/// and notification lists are exposed as additional tabs here so every inbox
/// stays one tap from the bottom navigation.
class MessagesPage extends StatelessWidget {
  const MessagesPage({super.key});

  @override
  Widget build(BuildContext context) {
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('消息'),
          bottom: const TabBar(
            tabs: [
              Tab(text: '私信'),
              Tab(text: '信箱'),
              Tab(text: '提醒'),
            ],
          ),
        ),
        body: const TabBarView(
          children: [
            ChatListScreen(embedded: true),
            MailScreen(embedded: true),
            ReferScreen(embedded: true),
          ],
        ),
      ),
    );
  }
}
