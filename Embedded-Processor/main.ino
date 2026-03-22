#include <Servo.h>

Servo esc1;
Servo esc2;

int escPin1 = 9;
int escPin2 = 10;
int linAcc1 = 11;
int linAcc2 = 12;
// int button1 = 2; removed these lines to establish 
// int button2 = 3;
// int button3 = 4;
// int button4 = 5;
int dir1 = 6;
int dir2 = 7;
// int potPin = A0;

int throttleMin = 0;   // ESC minimum throttle
int throttleMax = 2000;   // ESC maximum throttle

void setup() {
  // pinMode(button1,INPUT);
  // pinMode(button2,INPUT);
  // pinMode(button3,INPUT);
  // pinMode(button4,INPUT);
  Serial.begin(9600);

  esc1.attach(escPin1, throttleMin, throttleMax);
  esc2.attach(escPin2, throttleMin, throttleMax);

  // ---- ARM ESC ----
  Serial.println("Arming ESC...");
  esc1.writeMicroseconds(throttleMin);
  esc2.writeMicroseconds(throttleMin);
  delay(2000);  // most ESCs need 2 seconds of minimum throttle
  Serial.println("ESC Ready");
}

void loop() {
  // Check if we have enough bytes for a full packet
  if (Serial.available() >= 6) {
    // Look for the start byte
    if (Serial.read() == 0b10101000) {
      byte packet[4];
      Serial.readBytes(packet, 4); // Read the 4 data bytes

      // Check for the end byte
      if (Serial.read() == 0b00010101) {
        // Packet is valid, now parse the data bytes
        byte ljoyX = packet[0];
        byte ljoyY = packet[1];
        byte rjoyY = packet[2];
        byte rt = packet[3];

        // Map Left Joystick Y-axis to throttle (0-255 -> 1000-2000)
        int throttle = map(ljoyY, 0, 255, 1000, 2000);
        esc1.writeMicroseconds(throttle);
        esc2.writeMicroseconds(throttle);

        // For now, let's use Right Joystick Y-axis for the linear actuators
        // and Right Trigger for direction. This can be remapped later.
        digitalWrite(linAcc1, rjoyY > 128 ? HIGH : LOW);
        digitalWrite(linAcc2, rjoyY > 128 ? HIGH : LOW);
        digitalWrite(dir1, rt > 128 ? HIGH : LOW);
        digitalWrite(dir2, rt > 128 ? HIGH : LOW);

        // Debug output
        Serial.print("Throttle: ");
        Serial.print(throttle);
        Serial.print("  LJoyX: ");
        Serial.print(ljoyX);
        Serial.print("  RJoyY: ");
        Serial.print(rjoyY);
        Serial.print("  RT: ");
        Serial.println(rt);
      }
    }
  }
}