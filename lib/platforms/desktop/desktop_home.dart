import 'package:material_ui/material_ui.dart';
import 'package:byr_client/features/compose/compose_flow.dart';
import 'package:byr_client/features/message/messages_page.dart';
import 'package:byr_client/features/top_ten/top_ten_page.dart';
import 'package:byr_client/features/user/user_screen.dart';
import 'package:byr_client/platforms/desktop/forum_workspace.dart';

/// Desktop shell: navigation rail plus a multi-column forum workspace.
class DesktopHome extends StatefulWidget {
  const DesktopHome({super.key});

  @override
  State<DesktopHome> createState() => _DesktopHomeState();
}

class _DesktopHomeState extends State<DesktopHome> {
  int _index = 0;

  Future<void> _pickBoardAndCompose() async {
    final posted = await openComposeFlow(context);
    if (posted && mounted) setState(() => _index = 0);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: _index,
            onDestinationSelected: (value) {
              if (value == 2) {
                _pickBoardAndCompose();
                return;
              }
              setState(() => _index = value);
            },
            labelType: NavigationRailLabelType.all,
            leading: const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Text(
                'BYR',
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 18),
              ),
            ),
            destinations: const [
              NavigationRailDestination(
                icon: Icon(Icons.home_outlined),
                selectedIcon: Icon(Icons.home),
                label: Text('首页'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.whatshot_outlined),
                selectedIcon: Icon(Icons.whatshot),
                label: Text('十大'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.add_circle_outline),
                selectedIcon: Icon(Icons.add_circle),
                label: Text('发帖'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.chat_bubble_outline),
                selectedIcon: Icon(Icons.chat_bubble),
                label: Text('消息'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.person_outline),
                selectedIcon: Icon(Icons.person),
                label: Text('我'),
              ),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: IndexedStack(
              index: _index,
              children: const [
                ForumWorkspace(),
                TopTenPage(),
                SizedBox.shrink(),
                MessagesPage(),
                UserPage(),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
