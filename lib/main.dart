import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/bootstrap.dart';
import 'package:byr_client/app/byr_app.dart';

Future<void> main() async {
  final services = await bootstrapApplication();
  runApp(
    ByrApp(services: services, loggedIn: services.api.session.isAuthenticated),
  );
}
