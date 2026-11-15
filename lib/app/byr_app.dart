import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import 'package:byr_client/app/app_home.dart';
import 'package:byr_client/app/app_scope.dart';
import 'package:byr_client/app/app_services.dart';
import 'package:byr_client/features/login/login_screen.dart';

class ByrApp extends StatelessWidget {
  const ByrApp({super.key, required this.services, required this.loggedIn});

  final AppServices services;
  final bool loggedIn;

  @override
  Widget build(BuildContext context) {
    final session = services.api.session;
    return AppScope(
      services: services,
      child: ValueListenableBuilder<int>(
        valueListenable: session.themeMode,
        builder: (context, mode, _) => ValueListenableBuilder<double>(
          valueListenable: session.fontScale,
          builder: (context, scale, _) => MaterialApp(
            title: 'BYR',
            debugShowCheckedModeBanner: false,
            theme: _theme(Brightness.light),
            darkTheme: _theme(Brightness.dark),
            themeMode: ThemeMode.values[mode.clamp(0, 2).toInt()],
            builder: (context, child) {
              final content = MediaQuery(
                data: MediaQuery.of(context)
                    .copyWith(textScaler: TextScaler.linear(scale)),
                child: child ?? const SizedBox.shrink(),
              );
              final style = services.systemUiStyle(
                Theme.of(context).brightness,
              );
              return style == null
                  ? content
                  : AnnotatedRegion<SystemUiOverlayStyle>(
                      value: style,
                      child: content,
                    );
            },
            home: loggedIn ? const AppHome() : const LoginScreen(),
          ),
        ),
      ),
    );
  }

  ThemeData _theme(Brightness brightness) => ThemeData(
    colorScheme: ColorScheme.fromSeed(
      seedColor: const Color(0xFF0A5CBF),
      brightness: brightness,
    ),
    useMaterial3: true,
    appBarTheme: AppBarTheme(
      systemOverlayStyle: services.systemUiStyle(brightness),
    ),
  );
}
