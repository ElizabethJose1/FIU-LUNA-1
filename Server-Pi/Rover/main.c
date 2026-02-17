#define _POSIX_C_SOURCE 2024L // defines the POSIX version, most recent, to implement clock_gettime() function for millis() implementation
#include <stdint.h>
#include <stdio.h>
#include <time.h>

enum RoverState{
  IDLE,//0
  TELEOP, //1
  AUTO//2
}; // think of enum as describing ints with names EX: RED = 0

int currentState = IDLE;// the enum object creation, system will start in IDLE state in the setup() stage
unsigned long stateStartTime = 0;
unsigned long lastPrintTime = 0;

const unsigned long printInterval = 2500; // how often to print the remaining time in milliseconds, in this case every second

// writing functions because the functions must be above the setup and loop

millis() {

}

void changeState(enum RoverState newState){ // newState is just another instance for the rover state in function that is used in this function to change the state
  currentState = newState;
  stateStartTime = millis();
  lastPrintTime = millis(); // millis returns the number of milliseconds since the board began running the current program, stores it in stateStartTime so we know when state was entered/started so we can do countdowns based on that time.

  switch (currentState){
    case IDLE:
      printf("IDLE\n");
      break;
    case TELEOP:

      break;
    case AUTO:

      break;
  }
} 

void setup() {
  
}

void loop() {
  unsigned long now = millis(); //how long the program has been running minus the time when the state started which atp is 0 with stateStartTime updated every time the state changes.

  switch(currentState){
    case IDLE:
      if (now - lastPrintTime >= printInterval)
          lastPrintTime = now;
      break;

    case TELEOP:
      break;

    case AUTO:
      break;

  }
}

int main(void){
  setup();
  while(1){loop();}
}