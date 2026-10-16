#!/usr/bin/env python3
"""
Requiem Port Configuration Setup Tool
Automatically configures ports for the entire Requiem project.
"""

import yaml
import os
import sys
import subprocess
import socket
from pathlib import Path
from typing import Dict, Any, Optional


class RequiemPortConfigurator:
    def __init__(self, project_root: str = None):
        self.project_root = Path(project_root) if project_root else Path.cwd()
        self.config_path = self.project_root / "config" / "config.yaml"
        
        # Default ports
        self.defaults = {
            "requiem_server": 15153,
            "ollama": 11434,
            "mongodb": 27017,
            "dashboard_ws": 8081,
            "admin_portal": 8080,
            "landing_page": 3000,
        }
        
        # Current config
        self.config = {}
        
    def load_config(self) -> Dict[str, Any]:
        """Load existing config.yaml"""
        if self.config_path.exists():
            with open(self.config_path, 'r') as f:
                self.config = yaml.safe_load(f) or {}
        else:
            self.config = {}
        return self.config
    
    def save_config(self) -> None:
        """Save config.yaml with proper formatting"""
        self.config_path.parent.mkdir(parents=True, exist_ok=True)
        with open(self.config_path, 'w') as f:
            yaml.dump(self.config, f, default_flow_style=False, sort_keys=False)
    
    def check_port_available(self, port: int, host: str = "127.0.0.1") -> bool:
        """Check if a port is available"""
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.settimeout(1)
                result = s.connect_ex((host, port))
                return result != 0  # Port is available if connection fails
        except Exception:
            return False
    
    def find_available_port(self, start_port: int, host: str = "127.0.0.1") -> int:
        """Find next available port starting from start_port"""
        port = start_port
        while port < 65535:
            if self.check_port_available(port, host):
                return port
            port += 1
        raise RuntimeError(f"No available ports starting from {start_port}")
    
    def get_current_ports(self) -> Dict[str, int]:
        """Extract current ports from config"""
        ports = {}
        
        # Requiem server port
        if "server" in self.config and "port" in self.config["server"]:
            ports["requiem_server"] = self.config["server"]["port"]
        else:
            ports["requiem_server"] = self.defaults["requiem_server"]
        
        # Ollama port
        if "llm" in self.config and "providers" in self.config["llm"]:
            ollama_cfg = self.config["llm"]["providers"].get("ollama", {})
            base_url = ollama_cfg.get("base_url", "http://127.0.0.1:11434")
            # Extract port from URL
            try:
                ports["ollama"] = int(base_url.split(":")[-1])
            except:
                ports["ollama"] = self.defaults["ollama"]
        else:
            ports["ollama"] = self.defaults["ollama"]
        
        # MongoDB port
        if "database" in self.config and "mongodb" in self.config["database"]:
            uri = self.config["database"]["mongodb"].get("uri", "mongodb://127.0.0.1:27017")
            try:
                ports["mongodb"] = int(uri.split(":")[-1].split("/")[0])
            except:
                ports["mongodb"] = self.defaults["mongodb"]
        else:
            ports["mongodb"] = self.defaults["mongodb"]
        
        # Dashboard WebSocket (from config or default)
        ports["dashboard_ws"] = self.defaults["dashboard_ws"]
        ports["admin_portal"] = self.defaults["admin_portal"]
        ports["landing_page"] = self.defaults["landing_page"]
        
        return ports
    
    def configure_ports(self, interactive: bool = False) -> Dict[str, int]:
        """Configure all ports, finding available ones if needed"""
        self.load_config()
        current_ports = self.get_current_ports()
        new_ports = {}
        
        print("=== Requiem Port Configuration ===\n")
        
        for service, default_port in self.defaults.items():
            current = current_ports.get(service, default_port)
            
            if self.check_port_available(current):
                available = True
                status = "[OK] Available"
            else:
                available = False
                status = "[FAIL] In use"
            
            if interactive:
                print(f"\n{service}:")
                print(f"  Current: {current} ({status})")
                print(f"  Default: {default_port}")
                choice = input(f"  Use port {current}? [Y/n/new_port]: ").strip().lower()
                
                if choice == 'n':
                    new_port = int(input(f"  Enter new port for {service}: "))
                    new_ports[service] = new_port
                elif choice.isdigit():
                    new_ports[service] = int(choice)
                else:
                    new_ports[service] = current
            else:
                if not available:
                    # Auto-find next available port
                    new_port = self.find_available_port(current + 1)
                    print(f"  {service}: {current} in use -> {new_port}")
                    new_ports[service] = new_port
                else:
                    new_ports[service] = current
                    print(f"  {service}: {current} {status}")
        
        return new_ports
    
    def apply_ports(self, ports: Dict[str, int]) -> None:
        """Apply port configuration to config.yaml"""
        self.load_config()
        
        # Update Requiem server port
        if "server" not in self.config:
            self.config["server"] = {}
        self.config["server"]["port"] = ports.get("requiem_server", self.defaults["requiem_server"])
        
        # Update Ollama URL
        if "llm" not in self.config:
            self.config["llm"] = {}
        if "providers" not in self.config["llm"]:
            self.config["llm"]["providers"] = {}
        if "ollama" not in self.config["llm"]["providers"]:
            self.config["llm"]["providers"]["ollama"] = {}
        self.config["llm"]["providers"]["ollama"]["base_url"] = f"http://127.0.0.1:{ports.get('ollama', self.defaults['ollama'])}"
        
        # Update MongoDB URI
        if "database" not in self.config:
            self.config["database"] = {}
        if "mongodb" not in self.config["database"]:
            self.config["database"]["mongodb"] = {}
        self.config["database"]["mongodb"]["uri"] = f"mongodb://127.0.0.1:{ports.get('mongodb', self.defaults['mongodb'])}"
        
        # Add dashboard ports to config for reference
        self.config["dashboard_websocket_port"] = ports.get("dashboard_ws", self.defaults["dashboard_ws"])
        self.config["admin_portal_port"] = ports.get("admin_portal", self.defaults["admin_portal"])
        self.config["landing_page_port"] = ports.get("landing_page", self.defaults["landing_page"])
        
        self.save_config()
        print(f"\n[OK] Configuration saved to {self.config_path}")
    
    def update_firewall_rules(self, ports: Dict[str, int]) -> None:
        """Generate firewall commands for Windows"""
        print("\n=== Windows Firewall Rules ===")
        print("Run these commands as Administrator:\n")
        
        cmds = [
            f'netsh advfirewall firewall add rule name="Requiem Server" dir=in action=allow protocol=TCP localport={ports["requiem_server"]} profile=private,domain',
            f'netsh advfirewall firewall add rule name="Ollama API" dir=in action=allow protocol=TCP localport={ports["ollama"]} profile=private,domain',
            f'netsh advfirewall firewall add rule name="MongoDB" dir=in action=allow protocol=TCP localport={ports["mongodb"]} profile=private,domain',
            f'netsh advfirewall firewall add rule name="Requiem Dashboard WS" dir=in action=allow protocol=TCP localport={ports["dashboard_ws"]} profile=private,domain',
            f'netsh advfirewall firewall add rule name="Requiem Admin Portal" dir=in action=allow protocol=TCP localport={ports["admin_portal"]} profile=private,domain',
            f'netsh advfirewall firewall add rule name="Requiem Landing Page" dir=in action=allow protocol=TCP localport={ports["landing_page"]} profile=private,domain',
            f'netsh advfirewall firewall add rule name="Requiem Model Downloads" dir=out action=allow protocol=TCP remoteport=80,443 profile=private,domain',
        ]
        
        for cmd in cmds:
            print(f"  {cmd}")
        
        # Save to batch file
        bat_path = self.project_root / "control room" / "setup_firewall_auto.bat"
        bat_path.parent.mkdir(parents=True, exist_ok=True)
        with open(bat_path, 'w') as f:
            f.write("@echo off\n")
            f.write("REM Auto-generated firewall rules for Requiem\n")
            f.write("REM Run as Administrator\n\n")
            for cmd in cmds:
                f.write(cmd + "\n")
        print(f"\n[OK] Firewall script saved to {bat_path}")
    
    def update_control_room_scripts(self, ports: Dict[str, int]) -> None:
        """Update control room scripts with new ports"""
        cr_dir = self.project_root / "control room"
        cr_dir.mkdir(parents=True, exist_ok=True)
        
        # Update manage_server.bat
        manage_script = cr_dir / "manage_server.bat"
        if manage_script.exists():
            content = manage_script.read_text()
            content = content.replace(
                'set REQUIEM_PORT=15153',
                f'set REQUIEM_PORT={ports["requiem_server"]}'
            )
            content = content.replace(
                'set OLLAMA_PORT=11434',
                f'set OLLAMA_PORT={ports["ollama"]}'
            )
            manage_script.write_text(content)
            print(f"[OK] Updated {manage_script}")
        
        # Update run_modelmgr.bat
        run_script = cr_dir / "run_modelmgr.bat"
        if run_script.exists():
            content = run_script.read_text()
            content = content.replace(
                'set MODEL_MGR_PORT=11434',
                f'set MODEL_MGR_PORT={ports["ollama"]}'
            )
            run_script.write_text(content)
            print(f"[OK] Updated {run_script}")
    
    def print_summary(self, ports: Dict[str, int]) -> None:
        """Print configuration summary"""
        print("\n" + "="*50)
        print("REQUIEM PORT CONFIGURATION SUMMARY")
        print("="*50)
        print(f"\n  Requiem Server:     http://localhost:{ports['requiem_server']}")
        print(f"  Ollama API:         http://localhost:{ports['ollama']}")
        print(f"  MongoDB:            mongodb://localhost:{ports['mongodb']}")
        print(f"  Dashboard WS:       ws://localhost:{ports['dashboard_ws']}")
        print(f"  Admin Portal:       http://localhost:{ports['admin_portal']}")
        print(f"  Landing Page:       http://localhost:{ports['landing_page']}")
        
        print("\n  Config file:        config/config.yaml")
        print(f"  Firewall script:    control room/setup_firewall_auto.bat")
        print("="*50)


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="Requiem Port Configuration Tool")
    parser.add_argument("--project-root", default=".", help="Requiem project root directory")
    parser.add_argument("--interactive", "-i", action="store_true", help="Interactive mode")
    parser.add_argument("--check-only", "-c", action="store_true", help="Only check current ports")
    parser.add_argument("--non-interactive", "-n", action="store_true", help="Auto-configure without prompts")
    parser.add_argument("--firewall-only", action="store_true", help="Only generate firewall rules")
    
    args = parser.parse_args()
    
    configurator = RequiemPortConfigurator(args.project_root)
    
    if args.check_only:
        configurator.load_config()
        ports = configurator.get_current_ports()
        print("Current port configuration:")
        for service, port in ports.items():
            available = configurator.check_port_available(port)
            status = "✓" if available else "✗ IN USE"
            print(f"  {service}: {port} {status}")
        return
    
    if args.firewall_only:
        configurator.load_config()
        ports = configurator.get_current_ports()
        configurator.update_firewall_rules(ports)
        return
    
    ports = configurator.configure_ports(interactive=args.interactive)
    configurator.apply_ports(ports)
    configurator.update_firewall_rules(ports)
    configurator.update_control_room_scripts(ports)
    configurator.print_summary(ports)
    
    print("\n[OK] Port configuration complete!")
    print("\nNext steps:")
    print("  1. Run firewall script as Admin: control room/setup_firewall_auto.bat")
    print("  2. Start Ollama: ollama serve")
    print("  3. Start Requiem: control room/manage_server.bat")
    print("  4. Manage models: control room/run_modelmgr.bat")


if __name__ == "__main__":
    main()